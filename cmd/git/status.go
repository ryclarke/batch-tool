package git

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
)

func addStatusCmd() *cobra.Command {
	// statusCmd represents the git status command
	statusCmd := &cobra.Command{
		Use:   "status <repository> ...",
		Short: "Git status of each repository",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(call.Exec("git", "-c", "color.status=always", "status", "-sb")))
		},
	}

	return statusCmd
}
