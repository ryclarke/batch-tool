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
		Use:               "branch <repository>...",
		Aliases:           []string{"checkout"},
		Short:             "Checkout a new branch across repositories",
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
