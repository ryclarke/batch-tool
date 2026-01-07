package git

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
)

func addDiffCmd() *cobra.Command {
	// diffCmd represents the diff command
	diffCmd := &cobra.Command{
		Use:   "diff <repository>...",
		Short: "Git diff of each repository",
		Long: `Show git diff for each specified repository.

This command runs 'git diff' in each repository, displaying:
  - Line-by-line changes to tracked files
  - Additions and deletions with color coding
  - File paths and change locations

The diff shows unstaged changes in the working directory compared to the
index (staging area). Staged changes are not shown unless committed.

For reviewing staged changes, use 'git diff --cached' directly in the repo.

Use Cases:
  - Review changes before committing
  - Compare working directory to last commit
  - Identify what has changed across repositories
  - Debug unexpected file modifications`,
		Example: `  # Show diff for specific repositories
  batch-tool git diff repo1 repo2

  # Show diff for all backend services
  batch-tool git diff ~backend

  # Show diff with native output (for piping)
  batch-tool git diff -o native repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Exec("git", "diff"))
		},
	}

	return diffCmd
}
