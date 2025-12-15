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

// addMergeCmd initializes the pr merge command
func addMergeCmd() *cobra.Command {
	mergeCmd := &cobra.Command{
		Use:               "merge <repository> ...",
		Short:             "Merge accepted pull requests",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(utils.ValidateBranch, Merge))
		},
	}

	return mergeCmd
}

// Merge merges the pull request for the given repository.
func Merge(ctx context.Context, name string, ch chan<- string) error {
	viper := config.Viper(ctx)

	provider := scm.Get(ctx, viper.GetString(config.GitProvider), viper.GetString(config.GitProject))

	branch, err := utils.LookupBranch(ctx, name)
	if err != nil {
		return err
	}

	pr, err := provider.MergePullRequest(name, branch)
	if err != nil {
		return err
	}

	ch <- fmt.Sprintf("Merged pull request (#%d) %s\n", pr.Number, pr.Title)

	return nil
}
