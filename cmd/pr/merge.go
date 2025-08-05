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

	return mergeCmd
}

func mergePR(name string, ch chan<- string) error {
	provider := scm.Get(viper.GetString(config.GitProvider), viper.GetString(config.GitProject))

	branch, err := utils.LookupBranch(name)
	if err != nil {
		return err
	}

	pr, err := provider.MergePullRequest(name, branch)
	if err != nil {
		return err
	}

	ch <- fmt.Sprintf("Merged pull request (#%d) %s\n", pr["id"].(int), pr["title"].(string))

	return nil
}
