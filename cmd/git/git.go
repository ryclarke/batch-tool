package git

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/output"
)

// Cmd configures the root git command along with all subcommands and flags
func Cmd() *cobra.Command {
	gitCmd := &cobra.Command{
		Use:   "git <command> [flags] <repository>...",
		Short: "Manage git branches and commits",
		Long: `Manage git operations across multiple repositories.

This command provides a suite of git operations that can be executed across
multiple repositories simultaneously, including branch management, commits,
pushes, and status checks.`,
		Example: `  # Check status of repositories (default command)
  batch-tool git ~backend

  # Check status of repositories
  batch-tool git status repo1 repo2

  # Create new branches
  batch-tool git branch -b feature/new-api repo1 repo2

  # Commit changes with a common message
  batch-tool git commit -m "Fix bug" ~backend

  # Push changes to remote
  batch-tool git push repo1 repo2`,
		Args: cobra.MinimumNArgs(1),
	}

	gitCmd.AddCommand(
		addStatusCmd(),
		addBranchCmd(),
		addCommitCmd(),
		addDiffCmd(),
		addPushCmd(),
		addStashCmd(),
		addUpdateCmd(),
	)

	return gitCmd
}

// ValidateBranch returns an error if the current git branch is the default branch.
// If a branch name is provided, it checks against that branch instead.
func ValidateBranch(branch ...string) call.Func {
	return func(ctx context.Context, ch output.Channel) error {
		if len(branch) == 0 {
			// Get the default branch from the catalog if not provided
			branch = []string{catalog.GetBranchForRepo(ctx, ch.Name())}
		}

		cmd, err := call.Cmd(ctx, ch.Name(), "git", "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			return err
		}

		output, err := cmd.Output()
		if err != nil {
			return err
		}

		if strings.TrimSpace(string(output)) == strings.TrimSpace(branch[0]) {
			return fmt.Errorf("skipping operation - %s is the base branch", strings.TrimSpace(string(output)))
		}

		return nil
	}
}
