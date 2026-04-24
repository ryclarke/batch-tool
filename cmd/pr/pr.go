package pr

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	prTitleFlag        = "title"
	prDescriptionFlag  = "description"
	prReviewerFlag     = "reviewer"
	prTeamReviewerFlag = "team-reviewer"
	prDraftFlag        = "draft"
	prNoDraftFlag      = "no-" + prDraftFlag
)

// Cmd configures the root pr command along with all subcommands and flags
func Cmd() *cobra.Command {
	prCmd := &cobra.Command{
		Use:   "pr <repository>...",
		Short: "Manage pull requests using supported SCM provider APIs",
		Long: `Manage pull requests across multiple repositories using SCM provider APIs.

This command provides pull request management operations that integrate with
your source control management (SCM) provider.

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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Call root's persistent pre-run to initialize global flags for nested subcommands
			if cmd != cmd.Root() && cmd.Root().PersistentPreRunE != nil {
				if err := cmd.Root().PersistentPreRunE(cmd, args); err != nil {
					return err
				}
			}

			return utils.ValidateRequiredConfig(cmd.Context(), config.AuthToken)
		},
	}

	prCmd.AddCommand(
		addGetCmd(),
		addNewCmd(),
		addEditCmd(),
		addMergeCmd(),
	)

	return prCmd
}

func prOptions(ctx context.Context, name string, merge bool) scm.PROptions {
	viper := config.Viper(ctx)

	o := viper.Get(config.PrOptions)
	if o == nil {
		return scm.PROptions{}
	}

	opts, ok := o.(scm.PROptions)
	if !ok {
		return scm.PROptions{}
	}

	// Return only merge-related options if merge is true
	if merge {
		return scm.PROptions{Merge: opts.Merge}
	}

	opts.Merge = scm.PRMergeOptions{} // Clear merge options for non-merge operations
	opts.BaseBranch = catalog.GetBranchForRepo(ctx, name)

	return opts
}

func buildPROptions(cmd *cobra.Command) {
	viper := config.Viper(cmd.Context())

	// Build PR options from flags and config
	opts := scm.PROptions{
		Title:       viper.GetString(config.PrTitle),
		Description: viper.GetString(config.PrDescription),

		Reviewers:      viper.GetStringSlice(config.PrReviewers),
		TeamReviewers:  viper.GetStringSlice(config.PrTeamReviewers),
		ResetReviewers: viper.GetBool(config.PrResetReviewers),

		Merge: scm.PRMergeOptions{
			Method:         viper.GetString(config.PrMergeMethod),
			CheckMergeable: viper.GetBool(config.PrMergeCheck),
		},
	}

	// Set draft option if flag was explicitly provided
	if cmd.Flags().Changed(prDraftFlag) || cmd.Flags().Changed(prNoDraftFlag) {
		draft := viper.GetBool(config.PrDraft)
		opts.Draft = &draft
	}

	viper.Set(config.PrOptions, opts)
}

func printPRInfo(pr *scm.PullRequest, header string, verbose bool) string {
	var info strings.Builder

	// print custom header message if provided
	if header != "" {
		fmt.Fprintf(&info, "%s ", header)
	}

	// print common metadata onto title line
	if len(pr.TeamReviewers) > 0 {
		fmt.Fprintf(&info, "(PR #%d) %s %v %v\n", pr.Number, pr.Title, pr.Reviewers, pr.TeamReviewers)
	} else {
		fmt.Fprintf(&info, "(PR #%d) %s %v\n", pr.Number, pr.Title, pr.Reviewers)
	}

	if verbose && (pr.Branch != "" || pr.BaseBranch != "") {
		head, base := pr.Branch, pr.BaseBranch
		if head == "" {
			head = "???"
		}
		if base == "" {
			base = "???"
		}

		fmt.Fprintf(&info, "Branch: %s → %s\n", head, base)
	}

	// print description if verbose and description is not empty
	if verbose && pr.Description != "" {
		fmt.Fprintf(&info, "Description:\n%s\n", pr.Description)
	}

	return info.String()
}

func parseCommonPRFlags(cmd *cobra.Command) error {
	viper := config.Viper(cmd.Context())

	viper.BindPFlag(config.PrTitle, cmd.Flags().Lookup(prTitleFlag))
	viper.BindPFlag(config.PrDescription, cmd.Flags().Lookup(prDescriptionFlag))
	viper.BindPFlag(config.PrReviewers, cmd.Flags().Lookup(prReviewerFlag))
	viper.BindPFlag(config.PrTeamReviewers, cmd.Flags().Lookup(prTeamReviewerFlag))

	return utils.BindBoolFlags(cmd, config.PrDraft, prDraftFlag, prNoDraftFlag)
}

func buildCommonPRFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(prTitleFlag, "t", "", "pull request title")
	cmd.Flags().StringP(prDescriptionFlag, "d", "", "pull request description")
	cmd.Flags().StringSliceP(prReviewerFlag, "r", nil, "pull request reviewer (repeatable)")
	cmd.Flags().StringSliceP(prTeamReviewerFlag, "R", nil, "pull request team reviewer (repeatable)")
	utils.BuildBoolFlags(cmd, prDraftFlag, "", prNoDraftFlag, "", "mark pull request as a draft")
}
