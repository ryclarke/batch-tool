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

This command runs 'git status -sb' (short format with branch info) for each
repository, displaying:
  - Current branch name
  - Tracking information (ahead/behind remote)
  - Modified, staged, and untracked files

The output includes color formatting for better readability when displayed
in a terminal.

Use Cases:
  - Quick overview of repository states
  - Check for uncommitted changes
  - Verify branch synchronization with remote
  - Identify repositories that need attention`,
		Example: `  # Check status of specific repositories
  batch-tool git status repo1 repo2

  # Check all backend services
  batch-tool git status ~backend

  # Check all repositories (using label or pattern)
  batch-tool git status ~all

  # With native output for scripting
  batch-tool git status -o native repo1`,
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
