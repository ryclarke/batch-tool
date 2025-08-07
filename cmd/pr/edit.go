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

var noAppendReviewers bool

// addEditCmd initializes the pr edit command
func addEditCmd() *cobra.Command {
	editCmd := &cobra.Command{
		Use:   "edit <repository> ...",
		Short: "Update existing pull requests",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(utils.ValidateBranch, editPR))
		},
	}

	editCmd.Flags().BoolVar(&noAppendReviewers, "no-append", false, "don't append to the reviewer list")

	return editCmd
}

func editPR(name string, ch chan<- string) error {
	branch, err := utils.LookupBranch(name)
	if err != nil {
		return fmt.Errorf("failed to lookup branch for %s: %w", name, err)
	}

	provider := scm.Get(viper.GetString(config.GitProvider), viper.GetString(config.GitProject))

	pr, err := provider.UpdatePullRequest(name, branch, prTitle, prDescription, utils.LookupReviewers(name), !noAppendReviewers)
	if err != nil {
		return err
	}

	ch <- fmt.Sprintf("Updated pull request (#%d) %s %v\n", pr.Number, pr.Title, pr.Reviewers)

	return nil
}
