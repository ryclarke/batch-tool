package git

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	stashFlag   = "stash"
	noStashFlag = "no-" + stashFlag
)

func addUpdateCmd() *cobra.Command {
	// updateCmd represents the update command
	updateCmd := &cobra.Command{
		Use:   "update <repository>...",
		Short: "Update primary branch across repositories",
		Long: `Update the primary/default branch to the latest from remote.

This command performs the following operations for each repository:
  1. Checkout the default branch (main, master, develop, etc.)
  2. Pull the latest changes from the remote

With the --stash flag enabled (or git.stash-updates set in config), it will also:
  1. Stash uncommitted changes before updating (if any)
  2. Restore stashed changes after updating

WARNING: Without stash enabled, any uncommitted changes (staged or unstaged)
will be destroyed during the update process. Use --stash or set git.stash-updates
in your config to preserve your work.

The default branch name is determined from the repository catalog
configuration, which typically reads it from the git repository's
HEAD reference or uses a configured default.`,
		Example: `  # Update specific repositories
  batch-tool git update repo1 repo2

  # Update all repositories
  batch-tool git update ~all

  # Update with automatic stash/restore of uncommitted changes
  batch-tool git update --stash repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return utils.BindBoolFlags(cmd, config.StashUpdates, stashFlag, noStashFlag)
		},
		Run: func(cmd *cobra.Command, args []string) {
			if config.Viper(cmd.Context()).GetBool(config.StashUpdates) {
				call.Do(cmd, args, call.Wrap(StashPush, Update, StashPop))
			} else {
				call.Do(cmd, args, call.Wrap(Clean, Update))
			}
		},
	}

	utils.BuildBoolFlags(updateCmd, stashFlag, "", noStashFlag, "", "Automatically stash and restore uncommitted changes during update")

	return updateCmd
}

var (
	// Update checks out the default branch and pulls the latest changes.
	Update = call.Wrap(
		call.Exec("sh", "-c", "git checkout ${GIT_DEFAULT_BRANCH}"),
		call.Exec("git", "pull"),
	)

	// Clean resets any uncommitted changes, removes untracked files, and re-initializes submodules.
	Clean = call.Wrap(
		call.Exec("git", "reset", "--hard"),
		call.Exec("git", "clean", "-fd"),
		call.Exec("git", "submodule", "deinit", "-f", "."),
		call.Exec("git", "submodule", "update", "--init", "--recursive"),
	)
)
