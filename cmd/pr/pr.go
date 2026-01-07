package pr

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	prTitleFlag       = "title"
	prDescriptionFlag = "description"
	prReviewerFlag    = "reviewer"
)

// Cmd configures the root pr command along with all subcommands and flags
func Cmd() *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr [cmd] <repository>...",
		Short: "Manage pull requests using supported SCM provider APIs",
		Args:  cobra.MinimumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.PrTitle, cmd.PersistentFlags().Lookup(prTitleFlag))
			viper.BindPFlag(config.PrDescription, cmd.PersistentFlags().Lookup(prDescriptionFlag))
			viper.BindPFlag(config.PrReviewers, cmd.PersistentFlags().Lookup(prReviewerFlag))

			return utils.ValidateRequiredConfig(cmd.Context(), config.AuthToken)
		},
	}

	prCmd.PersistentFlags().StringP(prTitleFlag, "t", "", "pull request title")
	prCmd.PersistentFlags().StringP(prDescriptionFlag, "d", "", "pull request description")
	prCmd.PersistentFlags().StringSliceP(prReviewerFlag, "r", nil, "pull request reviewer, can be specified multiple times")

	defaultCmd := addGetCmd()
	prCmd.Run = defaultCmd.Run
	prCmd.AddCommand(
		defaultCmd,
		addNewCmd(),
		addEditCmd(),
		addMergeCmd(),
	)

	return prCmd
}

// lookupReviewers returns the list of reviewers for the given repository
func lookupReviewers(ctx context.Context, name string) []string {
	viper := config.Viper(ctx)

	// Use the provided list of reviewers
	if revs := viper.GetStringSlice(config.PrReviewers); len(revs) > 0 {
		return revs
	}

	// Use default reviewers for the given repository
	return viper.GetStringMapStringSlice(config.DefaultReviewers)[name]
}
