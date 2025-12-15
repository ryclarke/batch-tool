package git

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
)

func addStatusCmd() *cobra.Command {
	// statusCmd represents the git status command
	statusCmd := &cobra.Command{
		Use:               "status <repository>...",
		Short:             "Git status of each repository",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, Status)
		},
	}

	return statusCmd
}

// Status shows the git status for the given repository.
func Status(ctx context.Context, repo string, ch chan<- string) error {
	return call.Exec("git", "-c", "color.status=always", "status", "-sb")(ctx, repo, ch)
}
