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

var verifyMerge bool

// addMergeCmd initializes the pr merge command
func addMergeCmd() *cobra.Command {
	mergeCmd := &cobra.Command{
		Use:   "merge <repository> ...",
		Short: "Merge accepted pull requests",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(utils.ValidateBranch, mergePR))
		},
	}

	mergeCmd.Flags().BoolVar(&verifyMerge, "verify", false, "verify the pull request is mergeable before merging")
	mergeCmd.Flags().StringP("method", "m", "squash", "merge method to use")
	viper.BindPFlag(config.MergeMethod, mergeCmd.Flags().Lookup("method"))

	return mergeCmd
}

func mergePR(name string, ch chan<- string) error {
	provider := scm.Get(viper.GetString(config.GitProvider), viper.GetString(config.GitProject))

	branch, err := utils.LookupBranch(name)
	if err != nil {
		return err
	}

	switch viper.GetString(config.MergeMethod) {
	case "merge", "squash", "rebase":
		// valid methods
	default:
		return fmt.Errorf("invalid merge method: %s", viper.GetString(config.MergeMethod))
	}

	pr, err := provider.MergePullRequest(name, branch, verifyMerge)
	if err != nil {
		return err
	}

	ch <- fmt.Sprintf("Merged pull request (#%d) %s\n", pr.Number, pr.Title)

	return nil
}
