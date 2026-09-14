package git

import (
	"context"

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
		Use:     "branch -b <branch-name> [--discard] [--no-reset] <repository>...",
		Aliases: []string{"checkout"},
		Short:   "Checkout a new branch across repositories",
		Long: `Create and checkout a new branch across multiple repositories.

This command runs 'git checkout -B <branch>' in each specified repository,
which creates the branch if it doesn't exist, or resets it if it does. Pass
--no-reset (or set git.branch.reset to false) to use 'git checkout -b'
instead, which fails rather than discarding an existing branch.

Before creating the branch, the command automatically:
  1. Stashes uncommitted changes (unless --discard is used)
  2. Updates the primary/default branch to latest
  3. Creates the new branch from that updated state
  4. Restores stashed changes (if applicable)

This ensures all new branches start from a consistent, up-to-date baseline.`,
		Example: `  # Create a feature branch across repositories
  batch-tool git branch -b feature/add-auth repo1 repo2

  # Fail instead of resetting branches that already exist
  batch-tool git branch -b feature/add-auth --no-reset repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.Branch, cmd.Flags().Lookup(branchFlag))

			if err := utils.BindBoolFlags(cmd, config.GitBranchReset, resetFlag, noResetFlag); err != nil {
				return err
			}

			return utils.ValidateRequiredConfig(cmd.Context(), config.Branch)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var callFunc call.Func

			// Determine whether to stash or discard uncommitted changes
			if discard, err := cmd.Flags().GetBool(discardFlag); err == nil && discard {
				callFunc = call.Wrap(Update, Branch)
			} else {
				callFunc = call.Wrap(StashPush, Update, Branch, StashPop)
			}

			return call.Do(cmd, args, callFunc)
		},
	}

	branchCmd.Flags().StringP(branchFlag, "b", "", "branch name (required)")
	branchCmd.Flags().Bool(discardFlag, false, "discard uncommitted changes instead of stashing them")
	utils.BuildBoolFlags(branchCmd, resetFlag, "", noResetFlag, "", "reset the branch if it already exists")

	return branchCmd
}

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
