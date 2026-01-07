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
	branchFlag = "branch"
)

func addBranchCmd() *cobra.Command {
	// branchCmd represents the branch command
	branchCmd := &cobra.Command{
		Use:     "branch -b <branch-name> <repository>...",
		Aliases: []string{"checkout"},
		Short:   "Checkout a new branch across repositories",
		Long: `Create and checkout a new branch across multiple repositories.

This command runs 'git checkout -B <branch>' in each specified repository,
which creates the branch if it doesn't exist, or resets it if it does.

Before creating the branch, the command automatically:
  1. Updates the primary/default branch to latest
  2. Creates the new branch from that updated state

This ensures all repositories start from a consistent, up-to-date baseline.

The -B flag (force create) means:
  - If the branch exists, it will be reset to the current HEAD
  - If the branch doesn't exist, it will be created
  - This is safe for creating new feature branches

Use Cases:
  - Start new features across multiple services
  - Create consistent branch names for related changes
  - Ensure all repositories are on the same branch for coordination`,
		Example: `  # Create a feature branch across repositories
  batch-tool git branch -b feature/add-auth repo1 repo2

  # Create a branch for all backend services
  batch-tool git branch -b fix/database-query ~backend

  # Create a release branch
  batch-tool git branch -b release/v2.0 ~all

  # Short form using checkout alias
  batch-tool git checkout -b hotfix/urgent-fix repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.Branch, cmd.Flags().Lookup(branchFlag))

			return utils.ValidateRequiredConfig(cmd.Context(), config.Branch)
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(Update, Branch))
		},
	}

	branchCmd.Flags().StringP(branchFlag, "b", "", "branch name (required)")

	return branchCmd
}

// Branch checks out a new branch in the given repository.
func Branch(ctx context.Context, ch output.Channel) error {
	branch := config.Viper(ctx).GetString(config.Branch)

	return call.Exec("git", "checkout", "-B", branch)(ctx, ch)
}
