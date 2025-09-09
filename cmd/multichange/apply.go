package multichange

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

var (
	changesDir     string
	dryRun         bool
	targetRepos    []string
	targetLabels   []string
	canaryLabel    string
	diffBasedMatch bool
)

// applyCmd represents the apply command
func applyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply changes from source repository to target repositories",
		Long: `Apply changes from source repository to target repositories

This command applies changes that have been tested in a source repository to other
repositories. It expects a directory structure with old/new file pairs:

  changes/
    ├── path/to/file1.old
    ├── path/to/file1.new
    ├── path/to/file2.old
    └── path/to/file2.new

For each file pair, the command:
1. Compares the .old file with the corresponding file in target repositories
2. If they match exactly (by content hash), applies the .new file content
3. If --diff-based-match is enabled, uses diff parsing to apply only specific line changes
4. If --partial-match is enabled, uses code block matching as a fallback
5. Reports what changes were made or skipped

The diff-based matching is more precise and can handle cases where files have similar
but not identical content by parsing the actual differences and applying only the
specific changes that are needed.

SERVICE NAME MAPPING:
The command automatically transforms file paths to match service naming conventions
in target repositories. For example:
  - cmd/example-svc-server/main.go → cmd/destination-server/main.go (for destination repo)
  - cmd/other-service-svc-server/config.go → cmd/target-server/config.go (for target repo)

This allows changes from one service to be applied to other services with different
names but similar structure.

By default, uses the standard changes directory at $GOPATH/src/changes.

Examples:
  # Use standard changes directory with exact matching only
  batch-tool multichange apply --target-labels backend-services
  
  # Enable diff-based matching for more flexible change application
  batch-tool multichange apply --diff-based-match --target-labels backend-services
  
  # Use custom changes directory with both diff-based and partial matching
  batch-tool multichange apply --changes-dir ./changes --diff-based-match --partial-match --target-repos repo1,repo2,repo3 --dry-run`,
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

			if err := runApply(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&changesDir, "changes-dir", "", "Directory containing old/new file pairs (default: $GOPATH/src/changes)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be changed without making actual changes")
	cmd.Flags().BoolVar(&diffBasedMatch, "diff-based-match", true, "Enable enhanced diff-based matching (default: true)")
	cmd.Flags().StringSliceVar(&targetRepos, "target-repos", nil, "Specific repositories to apply changes to")
	cmd.Flags().StringSliceVar(&targetLabels, "target-labels", nil, "Repository labels to apply changes to")
	cmd.Flags().StringVar(&canaryLabel, "canary-label", "canary", "Label identifying the canary service")

	return cmd
}

// Change represents a file change with old and new content
type Change struct {
	RelativePath string
	OldFile      string
	NewFile      string
	OldHash      string
	NewHash      string
}

// runApply executes the apply command logic
func runApply() error {
	if !dryRun {
		fmt.Println("Applying changes from source repository to target repositories...")
	} else {
		fmt.Println("Dry run: Showing what changes would be applied...")
	}

	// Validate changes directory exists
	if _, err := os.Stat(changesDir); os.IsNotExist(err) {
		return fmt.Errorf("changes directory does not exist: %s", changesDir)
	}

	// Parse changes from the directory
	changes, err := parseChanges()
	if err != nil {
		return fmt.Errorf("failed to parse changes: %w", err)
	}

	if len(changes) == 0 {
		return fmt.Errorf("no valid change pairs found in %s", changesDir)
	}

	fmt.Printf("Found %d file changes to apply\n", len(changes))

	// Get target repositories
	repos, err := getTargetRepositories()
	if err != nil {
		return fmt.Errorf("failed to get target repositories: %w", err)
	}

	if len(repos) == 0 {
		return fmt.Errorf("no target repositories found")
	}

	fmt.Printf("Targeting %d repositories\n", len(repos))

	// Apply changes to each repository
	successCount := 0
	for _, repo := range repos {
		fmt.Printf("\n📦 Processing repository: %s\n", repo)

		repoSuccessCount, err := applyChangesToRepo(repo, changes)
		if err != nil {
			fmt.Printf("  ❌ Error processing %s: %v\n", repo, err)
			continue
		}

		if repoSuccessCount > 0 {
			successCount++
			fmt.Printf("  ✅ Applied %d changes to %s\n", repoSuccessCount, repo)
		} else {
			fmt.Printf("  ⏭️  No applicable changes for %s\n", repo)
		}
	}

	fmt.Printf("\n🎉 Summary: Applied changes to %d/%d repositories\n", successCount, len(repos))
	return nil
}

// createBackup creates a backup of the target file before applying changes
// Only creates backup if one doesn't already exist to preserve original state
func createBackup(targetFile, relativePath, repoName string) error {
	// Get the backup directory (sibling to changes directory)
	backupDir, err := getBackupDir()
	if err != nil {
		return fmt.Errorf("failed to get backup directory: %w", err)
	}

	// Create backup file path with repository name to avoid conflicts
	// Structure: backup/{repo-name}/{relative-path}
	backupFile := filepath.Join(backupDir, repoName, relativePath)

	// Check if backup already exists - if so, skip to preserve original
	if _, err := os.Stat(backupFile); err == nil {
		if dryRun {
			fmt.Printf("        Backup already exists, preserving original: %s\n", backupFile)
		}
		return nil // Don't overwrite existing backup
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(filepath.Dir(backupFile), 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy the current target file to backup location
	if err := copyFile(targetFile, backupFile); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	if dryRun {
		fmt.Printf("        Created backup: %s\n", backupFile)
	}

	return nil
} // getBackupDir returns the backup directory path (sibling to changes directory)
func getBackupDir() (string, error) {
	// Get changes directory parent
	changesParent := filepath.Dir(changesDir)

	// Create backup directory as sibling to changes
	backupDir := filepath.Join(changesParent, "backup")

	return backupDir, nil
}

// parseChanges scans the changes directory for old/new file pairs
func parseChanges() ([]Change, error) {
	var changes []Change
	changeMap := make(map[string]*Change)

	err := filepath.Walk(changesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path from changes directory
		relPath, err := filepath.Rel(changesDir, path)
		if err != nil {
			return err
		}

		// Check if this is an old or new file
		var basePath string
		var isOld, isNew bool

		if strings.HasSuffix(relPath, ".old") {
			basePath = strings.TrimSuffix(relPath, ".old")
			isOld = true
		} else if strings.HasSuffix(relPath, ".new") {
			basePath = strings.TrimSuffix(relPath, ".new")
			isNew = true
		} else {
			// Skip files that don't have .old or .new extension
			return nil
		}

		// Get or create change entry
		change, exists := changeMap[basePath]
		if !exists {
			change = &Change{RelativePath: basePath}
			changeMap[basePath] = change
		}

		// Set the appropriate file path and compute hash
		hash, err := computeFileHash(path)
		if err != nil {
			return fmt.Errorf("failed to compute hash for %s: %w", path, err)
		}

		if isOld {
			change.OldFile = path
			change.OldHash = hash
		} else if isNew {
			change.NewFile = path
			change.NewHash = hash
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to slice, only including complete pairs
	for _, change := range changeMap {
		if change.OldFile != "" && change.NewFile != "" {
			changes = append(changes, *change)
		} else {
			fmt.Printf("⚠️  Skipping incomplete change pair for %s (missing .old or .new file)\n", change.RelativePath)
		}
	}

	return changes, nil
}

// getTargetRepositories determines which repositories to target
func getTargetRepositories() ([]string, error) {
	var repos []string

	// If specific repos were provided, use those
	if len(targetRepos) > 0 {
		return targetRepos, nil
	}

	// If labels were provided, resolve them (prefix with ~ to indicate labels)
	if len(targetLabels) > 0 {
		var labelArgs []string
		for _, label := range targetLabels {
			labelArgs = append(labelArgs, "~"+label)
		}
		repoSet := catalog.RepositoryList(labelArgs...)
		repos = repoSet.ToSlice()
	} else {
		// Default to all repositories except canary
		allRepos := catalog.RepositoryList(viper.GetString(config.SuperSetLabel))
		canaryRepos := catalog.RepositoryList("~" + canaryLabel)

		targetSet := allRepos.Difference(canaryRepos)
		repos = targetSet.ToSlice()
	}

	return repos, nil
}

// transformPathForRepo transforms a file path to match the target repository's service naming convention
func transformPathForRepo(originalPath, repoName string) string {
	// Handle cmd/*-server pattern transformation
	if strings.Contains(originalPath, "/") && strings.Contains(originalPath, "-server/") {
		pathParts := strings.Split(originalPath, "/")

		// Look for cmd directory followed by *-server directory
		for i, part := range pathParts {
			if part == "cmd" && i+1 < len(pathParts) {
				serverDirPart := pathParts[i+1]
				if strings.HasSuffix(serverDirPart, "-server") {
					// Transform the server directory name based on the repository name
					newServerDir := transformServerDirName(serverDirPart, repoName)
					if newServerDir != serverDirPart {
						// Create new path with transformed server directory
						newPathParts := make([]string, len(pathParts))
						copy(newPathParts, pathParts)
						newPathParts[i+1] = newServerDir
						return strings.Join(newPathParts, "/")
					}
				}
			}
		}
	}

	return originalPath
}

// transformServerDirName transforms a server directory name based on the repository name
func transformServerDirName(serverDir, repoName string) string {
	// Pattern: cmd/{service-name}-server/main.go -> cmd/{repo-name}-server/main.go
	// Examples:
	// example-svc-server -> destination-server (for destination repo)
	// other-service-svc-server -> target-repo-server (for target-repo repo)

	if strings.HasSuffix(serverDir, "-server") {
		// Create new server directory name using the repo name
		return repoName + "-server"
	}

	return serverDir
}

// getDisplayPath returns the path to display in messages, showing transformation if it occurred
func getDisplayPath(transformedPath, originalPath string) string {
	if transformedPath != originalPath {
		return fmt.Sprintf("%s (transformed from %s)", transformedPath, originalPath)
	}
	return transformedPath
}

// applyChangesToRepo applies all changes to a specific repository
func applyChangesToRepo(repo string, changes []Change) (int, error) {
	repoPath := utils.RepoPath(repo)
	appliedCount := 0

	for _, change := range changes {
		// Transform the relative path to match the target repository's service name
		transformedPath := transformPathForRepo(change.RelativePath, repo)
		targetFile := filepath.Join(repoPath, transformedPath)

		// Check if target file exists
		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			// If transformed path doesn't exist, try the original path as fallback
			originalTargetFile := filepath.Join(repoPath, change.RelativePath)
			if _, err := os.Stat(originalTargetFile); os.IsNotExist(err) {
				if transformedPath != change.RelativePath {
					fmt.Printf("    ⏭️  %s (file not found in repo)\n", transformedPath)
					fmt.Printf("        💡 Also tried original path: %s (also not found)\n", change.RelativePath)
				} else {
					fmt.Printf("    ⏭️  %s (file not found in repo)\n", change.RelativePath)
				}
				if dryRun {
					fmt.Printf("        💡 File would be created if it existed in the repository\n")
				}
				continue
			} else {
				// Use original path if it exists
				targetFile = originalTargetFile
			}
		}

		// Compute hash of target file
		targetHash, err := computeFileHash(targetFile)
		if err != nil {
			return appliedCount, fmt.Errorf("failed to compute hash for %s: %w", targetFile, err)
		}

		// Check if changes have already been applied (target matches new file)
		if targetHash == change.NewHash {
			fmt.Printf("    ⏭️  %s (changes already applied)\n", getDisplayPath(transformedPath, change.RelativePath))
			if dryRun {
				fmt.Printf("        ✨ ALREADY APPLIED: Target file already matches the new version\n")
			}
			continue
		}

		// Show dry-run information
		if dryRun {
			showDryRunInfo(targetFile, change, targetHash)
		}

		// Compare with old file hash
		if targetHash != change.OldHash {
			if diffBasedMatch {
				// Try diff-based matching (more precise than hash matching)
				applied, err := tryDiffBasedMatch(targetFile, change, repo)
				if err != nil {
					fmt.Printf("    ⏭️  %s (diff-based match failed: %v)\n", getDisplayPath(transformedPath, change.RelativePath), err)
					if dryRun {
						showMatchFailureDebugInfo(targetFile, change, "diff-based", err)
					}
					continue
				}
				if applied {
					fmt.Printf("    ✅ %s (diff-based match)\n", getDisplayPath(transformedPath, change.RelativePath))
					appliedCount++
					if dryRun {
						showWhatWouldChange(targetFile, change, "diff-based match")
					}
					continue
				} else {
					fmt.Printf("    ⏭️  %s (diff-based match found no applicable changes)\n", getDisplayPath(transformedPath, change.RelativePath))
					if dryRun {
						showWhyNoMatch(targetFile, change)
					}
					continue
				}
			}
			fmt.Printf("    ⏭️  %s (file differs from expected old version)\n", getDisplayPath(transformedPath, change.RelativePath))
			if dryRun {
				fmt.Printf("        💡 Enable --diff-based-match for advanced matching\n")
				showHashMismatchDetails(targetFile, change)
			}
			continue
		}

		// Apply the change
		if !dryRun {
			// Create backup before applying changes
			if err := createBackup(targetFile, change.RelativePath, repo); err != nil {
				fmt.Printf("    ⚠️  Warning: Failed to create backup for %s: %v\n", getDisplayPath(transformedPath, change.RelativePath), err)
				// Continue with apply even if backup fails
			}

			if err := copyFile(change.NewFile, targetFile); err != nil {
				return appliedCount, fmt.Errorf("failed to copy %s to %s: %w", change.NewFile, targetFile, err)
			}
		}

		fmt.Printf("    ✅ %s\n", getDisplayPath(transformedPath, change.RelativePath))
		if dryRun {
			fmt.Printf("        ✨ EXACT MATCH: Would replace file content\n")
			showFileContentPreview(targetFile, change)
		}
		appliedCount++
	}

	return appliedCount, nil
}

// computeFileHash computes SHA256 hash of a file
func computeFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// showDryRunInfo displays detailed information about what would happen during dry run
func showDryRunInfo(targetFile string, change Change, targetHash string) {
	fmt.Printf("        DRY RUN: Analyzing %s\n", change.RelativePath)
	fmt.Printf("        Target file hash: %s\n", targetHash[:12]+"...")
	fmt.Printf("        Expected old hash: %s\n", change.OldHash[:12]+"...")

	if targetHash == change.OldHash {
		fmt.Printf("        ✅ Hash match - would apply exact replacement\n")
		showFileContentPreview(targetFile, change)
	} else {
		fmt.Printf("        ⚠️  Hash mismatch - will try advanced matching\n")
		showHashMismatchDetails(targetFile, change)
	}
}

// showWhatWouldChange shows what changes would be applied
func showWhatWouldChange(targetFile string, change Change, method string) {
	fmt.Printf("        🔄 WOULD CHANGE (%s):\n", method)
	showFileContentPreview(targetFile, change)
}

// showMatchFailureDebugInfo shows debugging information when matching fails
func showMatchFailureDebugInfo(targetFile string, change Change, method string, err error) {
	fmt.Printf("        🐛 DEBUG: %s matching failed\n", method)
	fmt.Printf("        📝 Error: %v\n", err)
	showFileComparisonSummary(targetFile, change)
}

// showWhyNoMatch explains why no match was found
func showWhyNoMatch(targetFile string, change Change) {
	fmt.Printf("        🤔 WHY NO MATCH:\n")
	showFileComparisonSummary(targetFile, change)
}

// showFileContentPreview shows a preview of the file changes
func showFileContentPreview(targetFile string, change Change) {
	// Read old and new content for comparison
	oldContent, err := os.ReadFile(change.OldFile)
	if err != nil {
		fmt.Printf("        ❌ Could not read old file: %v\n", err)
		return
	}

	newContent, err := os.ReadFile(change.NewFile)
	if err != nil {
		fmt.Printf("        ❌ Could not read new file: %v\n", err)
		return
	}

	// Show a diff-like preview
	oldLines := strings.Split(string(oldContent), "\n")
	newLines := strings.Split(string(newContent), "\n")

	fmt.Printf("        📋 Changes to apply:\n")
	showLineDiff(oldLines, newLines, 3) // Show up to 3 lines of context
}

// showHashMismatchDetails shows why hashes don't match
func showHashMismatchDetails(targetFile string, change Change) {
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		fmt.Printf("        ❌ Could not read target file: %v\n", err)
		return
	}

	oldContent, err := os.ReadFile(change.OldFile)
	if err != nil {
		fmt.Printf("        ❌ Could not read old file: %v\n", err)
		return
	}

	targetLines := strings.Split(string(targetContent), "\n")
	oldLines := strings.Split(string(oldContent), "\n")

	fmt.Printf("        📊 File comparison:\n")
	fmt.Printf("        📄 Target file has %d lines\n", len(targetLines))
	fmt.Printf("        📄 Expected old file has %d lines\n", len(oldLines))

	// Show some different lines as examples
	differences := 0
	for i := 0; i < len(targetLines) && i < len(oldLines) && differences < 3; i++ {
		if strings.TrimSpace(targetLines[i]) != strings.TrimSpace(oldLines[i]) {
			fmt.Printf("        🔀 Line %d differs:\n", i+1)
			fmt.Printf("           Target: %q\n", strings.TrimSpace(targetLines[i]))
			fmt.Printf("           Expected: %q\n", strings.TrimSpace(oldLines[i]))
			differences++
		}
	}

	if len(targetLines) != len(oldLines) {
		fmt.Printf("        📏 Line count difference: target has %d, expected %d\n",
			len(targetLines), len(oldLines))
	}
}

// showFileComparisonSummary shows a summary comparison between files
func showFileComparisonSummary(targetFile string, change Change) {
	targetContent, err := os.ReadFile(targetFile)
	if err != nil {
		fmt.Printf("        ❌ Could not read target file: %v\n", err)
		return
	}

	oldContent, err := os.ReadFile(change.OldFile)
	if err != nil {
		fmt.Printf("        ❌ Could not read old file: %v\n", err)
		return
	}

	targetLines := strings.Split(string(targetContent), "\n")
	oldLines := strings.Split(string(oldContent), "\n")

	// Calculate similarity metrics
	commonLines := 0
	for i := 0; i < len(targetLines) && i < len(oldLines); i++ {
		if strings.TrimSpace(targetLines[i]) == strings.TrimSpace(oldLines[i]) {
			commonLines++
		}
	}

	maxLines := len(targetLines)
	if len(oldLines) > maxLines {
		maxLines = len(oldLines)
	}

	similarity := float64(commonLines) / float64(maxLines) * 100

	fmt.Printf("        📊 File similarity: %.1f%% (%d/%d lines match)\n",
		similarity, commonLines, maxLines)
	fmt.Printf("        📄 Target: %d lines, Expected: %d lines\n",
		len(targetLines), len(oldLines))

	// Show some context about what's different
	if similarity < 90 {
		fmt.Printf("        💡 Consider using --diff-based-match for better matching\n")
	}
}

// showLineDiff shows a simplified diff between two sets of lines
func showLineDiff(oldLines, newLines []string, maxContext int) {
	shown := 0
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	for i := 0; i < maxLines && shown < maxContext*2; i++ {
		var oldLine, newLine string

		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}

		if oldLine != newLine {
			if oldLine != "" {
				fmt.Printf("           - %s\n", oldLine)
				shown++
			}
			if newLine != "" {
				fmt.Printf("           + %s\n", newLine)
				shown++
			}
		}
	}

	if shown >= maxContext*2 {
		fmt.Printf("           ... (more changes not shown)\n")
	}
}
