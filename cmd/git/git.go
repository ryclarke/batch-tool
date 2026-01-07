package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

// Cmd configures the root git command along with all subcommands and flags
func Cmd() *cobra.Command {
	gitCmd := &cobra.Command{
		Use:   "git <command> [flags] <repository>...",
		Short: "Manage git branches and commits",
		Long: `Manage git operations across multiple repositories.

This command provides a suite of git operations that can be executed across
multiple repositories simultaneously, including branch management, commits,
pushes, and status checks.

The default subcommand is 'status' if no subcommand is specified.

Available Subcommands:
  status   - Show git status for repositories
  branch   - Create and checkout new branches
  commit   - Commit changes across repositories
  push     - Push commits to remote repositories
  diff     - Show git diffs for repositories
  update   - Update primary branch to latest`,
		Example: `  # Check status of specific repositories
  batch-tool git status repo1 repo2

  # Check status of all backend repositories
  batch-tool git ~backend

  # Create a new branch across repositories
  batch-tool git branch -b feature/new-api repo1 repo2

  # Commit changes with a message
  batch-tool git commit -m "Fix bug" ~backend

  # Push changes to remote
  batch-tool git push repo1 repo2`,
		Args: cobra.MinimumNArgs(1),
	}

	defaultCmd := addStatusCmd()
	gitCmd.Run = defaultCmd.Run
	gitCmd.AddCommand(
		defaultCmd,
		addBranchCmd(),
		addCommitCmd(),
		addDiffCmd(),
		addPushCmd(),
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

		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = utils.RepoPath(ctx, ch.Name())
		output, err := cmd.Output()
		if err != nil {
			return err
		}

		if strings.TrimSpace(string(output)) == strings.TrimSpace(branch[0]) {
			return fmt.Errorf("skipping operation - %s is the base branch", output)
		}

		return nil
	}
}
