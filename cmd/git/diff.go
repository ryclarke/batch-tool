package git

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
)

const (
	cachedFlag = "cached"
)

func addDiffCmd() *cobra.Command {
	// diffCmd represents the diff command
	diffCmd := &cobra.Command{
		Use:   "diff [--cached] <repository>...",
		Short: "Git diff of each repository",
		Long: `Show git diff for each specified repository.

This command runs 'git diff' in each repository, displaying:
  - Line-by-line changes to tracked files
  - Additions and deletions with color coding
  - File paths and change locations

By default, the diff shows unstaged changes in the working directory
compared to the index (staging area). Staged changes are shown instead
if the --cached flag is used.`,
		Example: `  # Show diff for specific repositories
  batch-tool git diff repo1 repo2

  # Show diff of staged changes
  batch-tool git diff --cached ~backend`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			gitArgs := []string{"diff"}

			if cached, err := cmd.Flags().GetBool(cachedFlag); err == nil && cached {
				gitArgs = append(gitArgs, "--cached")
			}

			call.Do(cmd, args, call.Exec("git", gitArgs...))
		},
	}

	diffCmd.Flags().Bool(cachedFlag, false, "show diff of staged changes (cached)")

	return diffCmd
}
