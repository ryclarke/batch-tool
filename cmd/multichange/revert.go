package multichange

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

// revert command variables
var revertDryRun bool
var revertTargetRepos []string
var revertTargetLabels []string
var revertCanaryLabel string

func NewRevertCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revert",
		Short: "Revert previously applied changes back to their original state",
		Long: `Revert changes that were previously applied using the apply command.

This command restores files from backups that were created during the apply process.
The backup files are stored in a 'backup' directory alongside the 'changes' directory,
organized by service name to prevent conflicts.

REVERTING PROCESS:
The revert process works by:
1. Looking for service-specific backup files corresponding to the changes
2. Restoring the original files from the backup directory
3. Only reverting files that have backups available for that specific service

BACKUP LOCATION & STRUCTURE:
Backups are automatically created in: [changes-parent-dir]/backup/{service-name}/
- If changes are in ~/go/src/changes/, backups are in ~/go/src/backup/
- Each service gets its own backup subdirectory to prevent overwrites
- Example structure:
  backup/
  ├── service-a/
  │   ├── config/features.go
  │   └── cmd/service-a-svc-server/main.go
  ├── service-b/
  │   ├── config/features.go
  │   └── cmd/service-b-svc-server/main.go
  └── service-c/
      ├── config/features.go
      └── cmd/service-c-svc-server/main.go

SERVICE NAME MAPPING:
Just like the apply command, revert automatically handles service name transformations:
  - cmd/example-svc-server/main.go → cmd/destination-server/main.go (for destination repo)
  - Ensures the correct files are reverted in target repositories
  - Backups are stored using the original path structure for consistency

SAFETY FEATURES:
- Always use --dry-run first to see what would be reverted
- Only reverts files that have available backups for that specific service
- Preserves the original backup files for future use
- Shows clear messages about what can and cannot be reverted

Examples:
  # Preview what would be reverted
  batch-tool multichange revert --dry-run --target-labels backend-services
  
  # Revert changes in specific repositories
  batch-tool multichange revert --target-repos repo1,repo2 --dry-run
  
  # Actually perform the revert (remove --dry-run when ready)
  batch-tool multichange revert --target-labels backend-services`,
		Run: func(cmd *cobra.Command, args []string) {
			// If no changes-dir specified, use the standard location
			if changesDir == "" {
				var err error
				changesDir, err = getChangesDir()
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Error getting standard changes directory: %v\n", err)
					os.Exit(1)
				}
			}

			if err := runRevert(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&changesDir, "changes-dir", "", "Directory containing old/new file pairs (default: $GOPATH/src/changes)")
	cmd.Flags().BoolVar(&revertDryRun, "dry-run", false, "Show what would be reverted without making actual changes")
	cmd.Flags().StringSliceVar(&revertTargetRepos, "target-repos", nil, "Specific repositories to revert changes in")
	cmd.Flags().StringSliceVar(&revertTargetLabels, "target-labels", nil, "Repository labels to revert changes in")
	cmd.Flags().StringVar(&revertCanaryLabel, "canary-label", "canary", "Label identifying the canary service")

	return cmd
}

// runRevert executes the revert command logic
func runRevert() error {
	if !revertDryRun {
		fmt.Println("Reverting changes back to original state...")
	} else {
		fmt.Println("DRY RUN: Analyzing what would be reverted...")
	}

	fmt.Printf("Using changes directory: %s\n", changesDir)

	// Parse changes from the changes directory
	changes, err := parseChanges()
	if err != nil {
		return fmt.Errorf("failed to parse changes: %w", err)
	}

	if len(changes) == 0 {
		fmt.Println("No changes found to revert")
		return nil
	}

	fmt.Printf("Found %d change(s) to potentially revert\n\n", len(changes))

	// Get target repositories
	targetRepos, err := getTargetRepositoriesForRevert()
	if err != nil {
		return fmt.Errorf("failed to get target repositories: %w", err)
	}

	if len(targetRepos) == 0 {
		return fmt.Errorf("no target repositories specified or found")
	}

	fmt.Printf("Target repositories: %v\n\n", targetRepos)

	totalReverted := 0
	for _, repo := range targetRepos {
		fmt.Printf("🔄 Processing repository: %s\n", repo)

		revertedCount, err := revertChangesInRepo(repo, changes)
		if err != nil {
			fmt.Printf("❌ Error processing %s: %v\n", repo, err)
			continue
		}

		totalReverted += revertedCount
		if revertedCount > 0 {
			fmt.Printf("✅ Reverted %d file(s) in %s\n", revertedCount, repo)
		} else {
			fmt.Printf("No files reverted in %s\n", repo)
		}
		fmt.Println()
	}

	if revertDryRun {
		fmt.Printf("DRY RUN SUMMARY: Would revert %d file(s) across %d repository(ies)\n", totalReverted, len(targetRepos))
		fmt.Println("Remove --dry-run flag to actually perform the revert")
	} else {
		fmt.Printf("✅ REVERT COMPLETE: Successfully reverted %d file(s) across %d repository(ies)\n", totalReverted, len(targetRepos))
	}

	return nil
}

// revertChangesInRepo reverts changes in a specific repository using backup files
func revertChangesInRepo(repo string, changes []Change) (int, error) {
	repoPath := utils.RepoPath(repo)
	revertedCount := 0

	// Get backup directory
	backupDir, err := getRevertBackupDir()
	if err != nil {
		return 0, fmt.Errorf("failed to get backup directory: %w", err)
	}

	// Check if backup directory exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		fmt.Printf("    No backup directory found at %s\n", backupDir)
		fmt.Printf("        Backups are created automatically when apply command is used\n")
		return 0, nil
	}

	for _, change := range changes {
		// Transform path for this repository
		transformedPath := transformPathForRepo(change.RelativePath, repo)
		targetFile := filepath.Join(repoPath, transformedPath)

		// Check if target file exists
		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			// If transformed path doesn't exist, try the original path as fallback
			originalTargetFile := filepath.Join(repoPath, change.RelativePath)
			if _, err := os.Stat(originalTargetFile); os.IsNotExist(err) {
				if transformedPath != change.RelativePath {
					fmt.Printf("    %s (transformed from %s) (file not found in repo)\n", transformedPath, change.RelativePath)
				} else {
					fmt.Printf("    %s (file not found in repo)\n", change.RelativePath)
				}
				continue
			} else {
				// Use original path if that exists
				targetFile = originalTargetFile
				transformedPath = change.RelativePath
			}
		}

		// Check if backup file exists (with repository-specific path)
		backupFile := filepath.Join(backupDir, repo, change.RelativePath)
		if _, err := os.Stat(backupFile); os.IsNotExist(err) {
			fmt.Printf("    %s (no backup available)\n", getDisplayPath(transformedPath, change.RelativePath))
			if revertDryRun {
				fmt.Printf("        No backup found at %s\n", backupFile)
			}
			continue
		}

		if revertDryRun {
			fmt.Printf("    🔄 %s (would be reverted from backup)\n", getDisplayPath(transformedPath, change.RelativePath))
			fmt.Printf("        WOULD RESTORE: From backup %s\n", backupFile)
		} else {
			// Read backup content
			backupContent, err := os.ReadFile(backupFile)
			if err != nil {
				return revertedCount, fmt.Errorf("failed to read backup file %s: %w", backupFile, err)
			}

			// Write backup content to target file
			if err := os.WriteFile(targetFile, backupContent, 0644); err != nil {
				return revertedCount, fmt.Errorf("failed to restore file %s: %w", targetFile, err)
			}

			fmt.Printf("    ✅ %s (restored from backup)\n", getDisplayPath(transformedPath, change.RelativePath))
		}

		revertedCount++
	}

	return revertedCount, nil
}

// getBackupDir returns the backup directory path (sibling to changes directory)
func getRevertBackupDir() (string, error) {
	// Get changes directory parent
	changesParent := filepath.Dir(changesDir)

	// Create backup directory as sibling to changes
	backupDir := filepath.Join(changesParent, "backup")

	return backupDir, nil
}

// getTargetRepositoriesForRevert gets the list of target repositories for revert operation
func getTargetRepositoriesForRevert() ([]string, error) {
	var repos []string

	// If specific repos were provided, use those
	if len(revertTargetRepos) > 0 {
		return revertTargetRepos, nil
	}

	// If labels were provided, resolve them (prefix with ~ to indicate labels)
	if len(revertTargetLabels) > 0 {
		var labelArgs []string
		for _, label := range revertTargetLabels {
			labelArgs = append(labelArgs, "~"+label)
		}
		repoSet := catalog.RepositoryList(labelArgs...)
		repos = repoSet.ToSlice()
	} else {
		// Default to all repositories except canary
		allRepos := catalog.RepositoryList(viper.GetString(config.SuperSetLabel))
		canaryRepos := catalog.RepositoryList("~" + revertCanaryLabel)

		targetSet := allRepos.Difference(canaryRepos)
		repos = targetSet.ToSlice()
	}

	return repos, nil
}
