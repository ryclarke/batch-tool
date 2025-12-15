package git

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
)

func addUpdateCmd() *cobra.Command {
	// updateCmd represents the update command
	updateCmd := &cobra.Command{
		Use:               "update <repository>...",
		Short:             "Update primary branch across repositories",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, Update)
		},
	}

	return updateCmd
}

// Update checks out the default branch and pulls the latest changes.
func Update(ctx context.Context, repo string, ch chan<- string) error {
	viper := config.Viper(ctx)
	if err := call.Exec("git", "checkout", viper.GetString(config.SourceBranch))(ctx, repo, ch); err != nil {
		return err
	}

	return call.Exec("git", "pull")(ctx, repo, ch)
}
