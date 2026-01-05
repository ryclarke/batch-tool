package pr

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/cmd/git"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	forceFlag = "force"
)

// addMergeCmd initializes the pr merge command
func addMergeCmd() *cobra.Command {
	mergeCmd := &cobra.Command{
		Use:               "merge <repository>...",
		Short:             "Merge accepted pull requests",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return config.Viper(cmd.Context()).BindPFlag(config.PrForceMerge, cmd.Flags().Lookup(forceFlag))
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(git.ValidateBranch, Merge))
		},
	}

	mergeCmd.Flags().BoolP(forceFlag, "f", false, "attempt to merge without checking PR status")

	return mergeCmd
}

// Merge merges the pull request for the given repository.
func Merge(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	// Get project from repository metadata in catalog, fall back to default
	project := catalog.GetProjectForRepo(ctx, ch.Name())
	provider := scm.Get(ctx, viper.GetString(config.GitProvider), project)

	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return err
	}

	pr, err := provider.MergePullRequest(ch.Name(), branch, viper.GetBool(config.PrForceMerge))
	if err != nil {
		return err
	}

	fmt.Fprintf(ch, "Merged pull request (#%d) %s\n", pr.Number, pr.Title)

	return nil
}
