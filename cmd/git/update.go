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
  1. Stash uncommitted changes (if any)
  2. Checkout the default branch (main, master, develop, etc.)
  3. Pull the latest changes from the remote
  4. Restore stashed changes (if applicable)

The default branch name is determined from the repository catalog
configuration, which typically reads it from the git repository's
HEAD reference or uses a configured default.`,
		Example: `  # Update specific repositories
  batch-tool git update repo1 repo2

  # Update all repositories
  batch-tool git update ~all`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(StashPush, Update, StashPop))
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
