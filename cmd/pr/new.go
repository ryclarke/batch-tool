package pr

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/utils"
)

var allReviewers bool

// addNewCmd initializes the pr new command
func addNewCmd() *cobra.Command {
	newCmd := &cobra.Command{
		Use:   "new <repository> ...",
		Short: "Submit new pull requests",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(utils.ValidateBranch, newPR))
		},
	}

	newCmd.Flags().BoolVarP(&allReviewers, "all-reviewers", "a", false, "use all provided reviewers for a new PR")

	return newCmd
}

func newPR(name string, ch chan<- string) error {
	branch, err := utils.LookupBranch(name)
	if err != nil {
		return err
	}

	reviewers := utils.LookupReviewers(name)
	if len(reviewers) == 0 {
		// append placeholder to prevent NPE below
		reviewers = append(reviewers, "")
	}

	// remove all but the first reviewer by default
	if !allReviewers && len(reviewers) > 1 {
		reviewers = reviewers[:1]
	}

	provider := scm.Get(viper.GetString(config.GitProvider), viper.GetString(config.GitProject))
	pr, err := provider.OpenPullRequest(name, branch, prTitle, prDescription, reviewers)
	if err != nil {
		return err
	}

	ch <- fmt.Sprintf("New pull request (#%d) %s %v\n", pr.Number, pr.Branch, pr.Reviewers)

	return nil
}
