// Package git provides git operations commands for batch-tool.
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
	branchFlag  = "branch"
	discardFlag = "discard"
)

func addBranchCmd() *cobra.Command {
	// branchCmd represents the branch command
	branchCmd := &cobra.Command{
		Use:     "branch -b <branch-name> [--discard] <repository>...",
		Aliases: []string{"checkout"},
		Short:   "Checkout a new branch across repositories",
		Long: `Create and checkout a new branch across multiple repositories.

This command runs 'git checkout -B <branch>' in each specified repository,
which creates the branch if it doesn't exist, or resets it if it does.

Before creating the branch, the command automatically:
  1. Stashes uncommitted changes (unless --discard is used)
  2. Updates the primary/default branch to latest
  3. Creates the new branch from that updated state
  4. Restores stashed changes (if applicable)

This ensures all new branches start from a consistent, up-to-date baseline.`,
		Example: `  # Create a feature branch across repositories
  batch-tool git branch -b feature/add-auth repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.Branch, cmd.Flags().Lookup(branchFlag))

			return utils.ValidateRequiredConfig(cmd.Context(), config.Branch)
		},
		Run: func(cmd *cobra.Command, args []string) {
			var callFunc call.Func

			// Determine whether to stash or discard uncommitted changes
			if discard, err := cmd.Flags().GetBool(discardFlag); err == nil && discard {
				callFunc = call.Wrap(Update, Branch)
			} else {
				callFunc = call.Wrap(StashPush, Update, Branch, StashPop)
			}

			call.Do(cmd, args, callFunc)
		},
	}

	branchCmd.Flags().StringP(branchFlag, "b", "", "branch name (required)")
	branchCmd.Flags().Bool(discardFlag, false, "discard uncommitted changes instead of stashing them")

	return branchCmd
}

// Branch checks out a new branch in the given repository.
func Branch(ctx context.Context, ch output.Channel) error {
	branch := config.Viper(ctx).GetString(config.Branch)

	return call.Exec("git", "checkout", "-B", branch)(ctx, ch)
}
