package git

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	branchFlag = "branch"

	resetFlag   = "reset"
	noResetFlag = "no-" + resetFlag
)

func addBranchCmd() *cobra.Command {
	// branchCmd represents the branch command
	branchCmd := &cobra.Command{
		Use:     "branch -b <branch-name> [--no-reset] <repository>...",
		Aliases: []string{"checkout"},
		Short:   "Checkout a new branch across repositories",
		Long: `Create and checkout a new branch across multiple repositories.

This command runs 'git checkout -B <branch>' in each specified repository,
which creates the branch if it doesn't exist, or resets it if it does. Pass
--no-reset (or set git.branch.reset to false) to use 'git checkout -b'
instead, which fails rather than discarding an existing branch.

Before creating the branch, the command automatically:
  1. Stashes uncommitted changes
  2. Updates the primary/default branch to latest
  3. Creates the new branch from that updated state
  4. Restores stashed changes onto the new branch

This ensures all new branches start from a consistent, up-to-date baseline.
To start from a clean worktree instead, discard local changes first with
'batch-tool git update --discard'.`,
		Example: `  # Create a feature branch across repositories
  batch-tool git branch -b feature/add-auth repo1 repo2

  # Fail instead of resetting branches that already exist
  batch-tool git branch -b feature/add-auth --no-reset repo1 repo2

  # Throw away local changes, then branch from the updated default branch
  batch-tool git update --discard repo1 repo2
  batch-tool git branch -b feature/add-auth repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Flags().Changed(discardFlag) {
				return errBranchDiscardRemoved
			}

			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.Branch, cmd.Flags().Lookup(branchFlag))

			if err := utils.BindBoolFlags(cmd, config.GitBranchReset, resetFlag, noResetFlag); err != nil {
				return err
			}

			return utils.ValidateRequiredConfig(cmd.Context(), config.Branch)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return call.Do(cmd, args, call.Wrap(StashPush, Update, Branch, StashPop))
		},
	}

	branchCmd.Flags().StringP(branchFlag, "b", "", "branch name (required)")
	utils.BuildBoolFlags(branchCmd, resetFlag, "", noResetFlag, "", "reset the branch if it already exists")

	// --discard is retained only to point callers at its replacement.
	branchCmd.Flags().Bool(discardFlag, false, "removed: run 'git update --discard' before branching")
	_ = branchCmd.Flags().MarkHidden(discardFlag)

	return branchCmd
}

var errBranchDiscardRemoved = errors.New("--discard was removed from 'git branch'; run 'batch-tool git update --discard <repository>...' first to discard local changes, then create the branch")

// Branch checks out a new branch in the given repository. Existing branches are
// reset unless git.branch.reset is disabled, in which case the checkout fails instead.
func Branch(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	create := "-b"
	if viper.GetBool(config.GitBranchReset) {
		create = "-B"
	}

	return call.Exec("git", "checkout", create, viper.GetString(config.Branch))(ctx, ch)
}
