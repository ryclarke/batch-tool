package multichange

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	extractOutputDir string
	gitRef           string
	baseBranch       string
	repoPath         string
)

// extractCmd represents the extract command
func extractCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract file changes from a git repository for later application",
		Long: `Extract file changes from a git repository for later application

This command extracts changed files from a specified git repository and creates old/new 
file pairs that can be used with the 'apply' command.

Common use cases:
1. Extract uncommitted changes (working directory vs last commit)
2. Extract committed changes (between two git references)

The command will:
1. Use the specified repository path
2. Find files that have changed
3. Extract both the old and new versions of each file
4. Save them as .old/.new pairs in the output directory

By default, saves extracted changes to $GOPATH/src/changes.

Examples:
  # Extract uncommitted changes to standard location
  batch-tool multichange extract --repo-path /path/to/repo

  # Extract uncommitted changes to custom location
  batch-tool multichange extract --repo-path /path/to/repo --output-dir ./changes

  # Extract changes between main branch and current HEAD  
  batch-tool multichange extract --repo-path /path/to/repo --base-branch main --git-ref HEAD

  # Extract changes between two specific commits
  batch-tool multichange extract --repo-path /path/to/repo --base-branch abc123 --git-ref def456`,
		Run: func(cmd *cobra.Command, args []string) {
			// If no output-dir specified, use the standard location
			if extractOutputDir == "" {
				var err error
				extractOutputDir, err = getChangesDir()
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Error getting standard changes directory: %v\n", err)
					os.Exit(1)
				}
			}

			if err := runExtract(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo-path", "", "Path to the git repository with changes (required)")
	cmd.Flags().StringVar(&extractOutputDir, "output-dir", "", "Directory to save extracted file pairs (default: $GOPATH/src/changes)")
	cmd.Flags().StringVar(&gitRef, "git-ref", "", "Git reference to compare to (leave empty for working directory changes)")
	cmd.Flags().StringVar(&baseBranch, "base-branch", "HEAD", "Base branch/commit to compare from")

	cmd.MarkFlagRequired("repo-path")

	return cmd
}

// runExtract executes the extract command logic
func runExtract() error {
	fmt.Println("Extracting changes from specified repository...")

	// Validate repository path exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return fmt.Errorf("repository path does not exist: %s", repoPath)
	}

	// Check if it's a git repository
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository: %s (no .git directory found)", repoPath)
	}

	fmt.Printf("Using repository: %s\n", repoPath)

	// Create output directory
	if err := os.MkdirAll(extractOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Determine what type of changes to extract
	var changeDescription string
	if gitRef == "" {
		changeDescription = fmt.Sprintf("working directory vs %s", baseBranch)
	} else {
		changeDescription = fmt.Sprintf("%s vs %s", baseBranch, gitRef)
	}
	fmt.Printf("Looking for changes: %s\n", changeDescription)

	// Get list of changed files
	changedFiles, err := getChangedFiles(repoPath, baseBranch, gitRef)
	if err != nil {
		return fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(changedFiles) == 0 {
		fmt.Printf("No file changes found (%s)\n", changeDescription)
		return nil
	}

	fmt.Printf("Found %d changed files\n", len(changedFiles))

	// Extract each changed file
	successCount := 0
	for _, file := range changedFiles {
		if err := extractFileChanges(repoPath, file, baseBranch, gitRef); err != nil {
			fmt.Printf("  ❌ Failed to extract %s: %v\n", file, err)
			continue
		}
		fmt.Printf("  ✅ Extracted %s\n", file)
		successCount++
	}

	fmt.Printf("\nSuccessfully extracted %d/%d files to %s\n", successCount, len(changedFiles), extractOutputDir)
	fmt.Printf("You can now use 'batch-tool multichange apply --changes-dir %s' to apply these changes\n", extractOutputDir)

	return nil
}

// getChangedFiles returns a list of files that changed between two git references or working directory
func getChangedFiles(repoPath, base, target string) ([]string, error) {
	var cmd *exec.Cmd

	if target == "" {
		// Compare working directory with base
		cmd = exec.Command("git", "diff", "--name-only", base)
	} else {
		// Compare two git references
		cmd = exec.Command("git", "diff", "--name-only", base, target)
	}
	cmd.Dir = repoPath

	// Execute the command and capture output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	// Parse the output
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, line := range lines {
		if line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

// extractFileChanges extracts the old and new versions of a file
func extractFileChanges(repoPath, file, base, target string) error {
	// Extract old version (from base)
	oldContent, err := getFileAtRef(repoPath, file, base)
	if err != nil {
		return fmt.Errorf("failed to get old version: %w", err)
	}

	// Extract new version (from target)
	newContent, err := getFileAtRef(repoPath, file, target)
	if err != nil {
		return fmt.Errorf("failed to get new version: %w", err)
	}

	// Create output file paths
	oldFile := filepath.Join(extractOutputDir, file+".old")
	newFile := filepath.Join(extractOutputDir, file+".new")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(oldFile), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write old version
	if err := os.WriteFile(oldFile, []byte(oldContent), 0644); err != nil {
		return fmt.Errorf("failed to write old file: %w", err)
	}

	// Write new version
	if err := os.WriteFile(newFile, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write new file: %w", err)
	}

	return nil
}

// getFileAtRef gets the content of a file at a specific git reference or working directory
func getFileAtRef(repoPath, file, ref string) (string, error) {
	if ref == "" {
		// Read from working directory
		filePath := filepath.Join(repoPath, file)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to read working directory file: %w", err)
		}
		return string(content), nil
	}

	// Read from git reference
	cmd := exec.Command("git", "show", fmt.Sprintf("%s:%s", ref, file))
	cmd.Dir = repoPath

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git show failed: %w", err)
	}

	return string(output), nil
}
