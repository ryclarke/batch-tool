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
	prDraftFlag       = "draft"
	prNoDraftFlag     = "no-" + prDraftFlag
)

// Cmd configures the root pr command along with all subcommands and flags
func Cmd() *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr <repository>...",
		Short: "Manage pull requests using supported SCM provider APIs",
		Long: `Manage pull requests across multiple repositories using SCM provider APIs.

This command provides pull request management operations that integrate with
your source control management (SCM) provider. If run without a subcommand it
defaults to 'pr get' to retrieve pull request information.

The active provider is configured in your batch-tool configuration file along with
authentication tokens. GitHub and Bitbucket are currently supported.

Authentication:
  Requires an authentication token configured for your SCM provider.
  Set this in your configuration file or via environment variables.

Branch Validation:
  PR commands validate that you're not on the default branch before executing.
  Use feature branches for pull requests.`,
		Example: `  # Get PR information
  batch-tool pr get repo1 repo2

  # Create new PRs with title and reviewers
  batch-tool pr new -t "Add feature" -r alice -r bob repo1 repo2

  # Update existing PRs
  batch-tool pr edit -t "Updated title" -d "New description" repo1

  # Merge approved PRs
  batch-tool pr merge repo1 repo2`,
		Args: cobra.MinimumNArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.PrTitle, cmd.Flags().Lookup(prTitleFlag))
			viper.BindPFlag(config.PrDescription, cmd.Flags().Lookup(prDescriptionFlag))
			viper.BindPFlag(config.PrReviewers, cmd.Flags().Lookup(prReviewerFlag))

			if err := utils.BindBoolFlags(cmd, config.PrDraft, prDraftFlag, prNoDraftFlag); err != nil {
				return err
			}

			return utils.ValidateRequiredConfig(cmd.Context(), config.AuthToken)
		},
	}

	prCmd.PersistentFlags().StringP(prTitleFlag, "t", "", "pull request title")
	prCmd.PersistentFlags().StringP(prDescriptionFlag, "d", "", "pull request description")
	prCmd.PersistentFlags().StringSliceP(prReviewerFlag, "r", nil, "pull request reviewer (repeatable)")
	utils.BuildBoolFlags(prCmd, prDraftFlag, "", prNoDraftFlag, "", "mark pull request as a draft")

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
