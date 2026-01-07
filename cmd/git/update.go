package git

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/output"
)

func addUpdateCmd() *cobra.Command {
	// updateCmd represents the update command
	updateCmd := &cobra.Command{
		Use:   "update <repository>...",
		Short: "Update primary branch across repositories",
		Long: `Update the primary/default branch to the latest from remote.

This command performs two operations for each repository:
  1. Checkout the default branch (main, master, develop, etc.)
  2. Pull the latest changes from the remote

The default branch name is determined from the repository catalog
configuration, which typically reads it from the git repository's
HEAD reference or uses a configured default.

This is commonly used:
  - Before starting new feature branches
  - To sync local repositories with remote state
  - After merging pull requests
  - To ensure you have the latest baseline

Note: This will fail if the working directory has uncommitted changes.
Commit or stash changes before updating.

Use Cases:
  - Sync repositories before starting new work
  - Update after PRs are merged remotely
  - Ensure consistent baseline across repositories
  - Prepare for creating new branches`,
		Example: `  # Update specific repositories
  batch-tool git update repo1 repo2

  # Update all backend services
  batch-tool git update ~backend

  # Update all repositories
  batch-tool git update ~all

  # Update synchronously (one at a time)
  batch-tool git update --sync repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, Update)
		},
	}

	return updateCmd
}

// Update checks out the default branch and pulls the latest changes.
func Update(ctx context.Context, ch output.Channel) error {
	if err := call.Exec("git", "checkout", catalog.GetBranchForRepo(ctx, ch.Name()))(ctx, ch); err != nil {
		return err
	}

	return call.Exec("git", "pull")(ctx, ch)
}
