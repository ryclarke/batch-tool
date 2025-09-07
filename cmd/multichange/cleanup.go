package multichange

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func cleanupCmd() *cobra.Command {
	var changesDir string
	var cleanupBackup bool
	var cleanupChanges bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove files from the changes and/or backup directories",
		Long: `Remove files from the changes and/or backup directories

This command removes files from the changes directory and/or backup directory.
By default, it only cleans up the changes directory to maintain backward compatibility.

The command will:
1. Remove files from the specified directory(ies)
2. Keep the directory structure itself (empty)
3. Report what was removed

CLEANUP OPTIONS:
- Default: Clean changes directory only (maintains existing behavior)
- --backup: Clean backup directory only
- --changes: Clean changes directory only (explicit)
- --backup --changes: Clean both directories

By default, uses the standard changes directory at $GOPATH/src/changes.
Backup directory is automatically detected as sibling to changes directory.

Examples:
  # Clean changes directory (default behavior)
  batch-tool multichange cleanup
  
  # Clean backup directory only
  batch-tool multichange cleanup --backup
  
  # Clean both changes and backup directories
  batch-tool multichange cleanup --changes --backup
  
  # Use custom changes directory  
  batch-tool multichange cleanup --changes-dir ./changes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If no changes-dir specified, use the standard location
			if changesDir == "" {
				var err error
				changesDir, err = getChangesDir()
				if err != nil {
					return fmt.Errorf("failed to get standard changes directory: %w", err)
				}
			}

			// If no specific cleanup flags set, default to changes only (backward compatibility)
			if !cleanupBackup && !cleanupChanges {
				cleanupChanges = true
			}

			return runCleanup(changesDir, cleanupChanges, cleanupBackup)
		},
	}

	cmd.Flags().StringVar(&changesDir, "changes-dir", "", "Directory containing old/new file pairs (default: $GOPATH/src/changes)")
	cmd.Flags().BoolVar(&cleanupBackup, "backup", false, "Clean the backup directory")
	cmd.Flags().BoolVar(&cleanupChanges, "changes", false, "Clean the changes directory")

	return cmd
}

func runCleanup(changesDir string, cleanupChanges, cleanupBackup bool) error {
	var totalRemoved int

	if cleanupChanges {
		if err := cleanupDirectory(changesDir, "changes"); err != nil {
			return err
		}
		totalRemoved++
	}

	if cleanupBackup {
		// Get backup directory (sibling to changes directory)
		changesParent := filepath.Dir(changesDir)
		backupDir := filepath.Join(changesParent, "backup")

		if err := cleanupDirectory(backupDir, "backup"); err != nil {
			return err
		}
		totalRemoved++
	}

	if totalRemoved == 0 {
		fmt.Println("No cleanup operations specified")
	}

	return nil
}

func cleanupDirectory(dirPath, dirType string) error {
	// Check if the directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		fmt.Printf("%s directory does not exist: %s\n", dirType, dirPath)
		return nil
	}

	// Read the directory contents
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read %s directory: %w", dirType, err)
	}

	if len(entries) == 0 {
		fmt.Printf("%s directory %s is already empty\n", dirType, dirPath)
		return nil
	}

	// Remove all files and subdirectories
	var removed []string
	for _, entry := range entries {
		entryPath := filepath.Join(dirPath, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entryPath, err)
		}
		removed = append(removed, entry.Name())
	}

	// Report what was removed
	fmt.Printf("✅ Removed %d items from %s directory (%s):\n", len(removed), dirType, dirPath)
	for _, item := range removed {
		fmt.Printf("  - %s\n", item)
	}

	return nil
}
