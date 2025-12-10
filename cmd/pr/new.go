package pr

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	allReviewersFlag = "all-reviewers"
)

// addNewCmd initializes the pr new command
func addNewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:               "new <repository>...",
		Short:             "Submit new pull requests",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.PrAllReviewers, cmd.Flags().Lookup(allReviewersFlag))

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(utils.ValidateBranch, New))
		},
	}

	newCmd.Flags().BoolP(allReviewersFlag, "a", false, "use all provided reviewers for a new PR (default: only the first)")

	return newCmd
}

// New creates a new pull request for the given repository.
func New(ctx context.Context, name string, ch chan<- string) error {
	viper := config.Viper(ctx)

	branch, err := utils.LookupBranch(ctx, name)
	if err != nil {
		return err
	}

	reviewers := lookupReviewers(ctx, name)
	if len(reviewers) == 0 {
		// append placeholder to prevent NPE below
		reviewers = append(reviewers, "")
	}

	// remove all but the first reviewer by default
	if !viper.GetBool(config.PrAllReviewers) && len(reviewers) > 1 {
		reviewers = reviewers[:1]
	}

	provider := scm.Get(ctx, viper.GetString(config.GitProvider), viper.GetString(config.GitProject))
	pr, err := provider.OpenPullRequest(name, branch, viper.GetString(config.PrTitle), viper.GetString(config.PrDescription), reviewers)
	if err != nil {
		return err
	}

	ch <- fmt.Sprintf("New pull request (#%d) %s %v\n", pr.Number, pr.Branch, pr.Reviewers)

	return nil
}
