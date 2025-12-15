package git

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	forceFlag = "force"
)

func addPushCmd() *cobra.Command {
	// pushCmd represents the push command
	pushCmd := &cobra.Command{
		Use:               "push <repository> ...",
		Short:             "Push committed code changes to remote",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRun: func(cmd *cobra.Command, _ []string) {
			viper := config.Viper(cmd.Context())
			viper.BindPFlag(config.CommitAmend, cmd.Flags().Lookup(forceFlag))
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(utils.ValidateBranch, Push))
		},
	}

	pushCmd.Flags().BoolP(forceFlag, "f", false, "overwrite remote with local changes")

	return pushCmd
}

// Push committed changes to the remote repository.
func Push(ctx context.Context, name string, ch chan<- string) error {
	viper := config.Viper(ctx)
	branch, err := utils.LookupBranch(ctx, name)
	if err != nil {
		return err
	}

	args := []string{"push", "-u", "origin", branch}
	if viper.GetBool(config.CommitAmend) {
		args = append(args, "-f")
	}

	return call.Exec("git", args...)(ctx, name, ch)
}
