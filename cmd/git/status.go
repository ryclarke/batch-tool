package git

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/output"
)

func addStatusCmd() *cobra.Command {
	// statusCmd represents the git status command
	statusCmd := &cobra.Command{
		Use:   "status <repository>...",
		Short: "Git status of each repository",
		Long: `Show the git status for each specified repository.

This command fetches the status for each repository, displaying:
  - Current branch name
  - Tracking information (ahead/behind remote)
  - Modified, staged, and untracked files`,
		Example: `  # Check status of specific repositories
  batch-tool git status repo1 repo2

  # Check all repositories
  batch-tool git status ~all`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, Status)
		},
	}

	return statusCmd
}

// Status shows the git status for the given repository.
func Status(ctx context.Context, ch output.Channel) error {
	return call.Exec("git", "-c", "color.status=always", "status", "-sb")(ctx, ch)
}
