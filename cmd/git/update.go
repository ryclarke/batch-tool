package git

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

const (
	stashFlag   = "stash"
	noStashFlag = "no-" + stashFlag

	pullStrategyFlag = "pull-strategy"
)

// Pull strategies for the update command.
const (
	// PullDefault defers to the repository's own pull configuration.
	PullDefault = "default"
	// PullFFOnly refuses to pull when a merge commit would be required.
	PullFFOnly = "ff-only"
	// PullRebase rebases local commits onto the upstream branch.
	PullRebase = "rebase"
	// PullMerge always creates a merge commit when histories diverge.
	PullMerge = "merge"
)

// AvailablePullStrategies lists the valid values for the --pull-strategy flag.
var AvailablePullStrategies = []string{PullDefault, PullFFOnly, PullRebase, PullMerge}

func addUpdateCmd() *cobra.Command {
	// updateCmd represents the update command
	updateCmd := &cobra.Command{
		Use:   "update [--stash] [--pull-strategy <strategy>] <repository>...",
		Short: "Update primary branch across repositories",
		Long: `Update the primary/default branch to the latest from remote.

This command performs the following operations for each repository:
  1. Checkout the default branch (main, master, develop, etc.)
  2. Pull the latest changes from the remote

With the --stash flag enabled (or git.update.stash set in config), it will also:
  1. Stash uncommitted changes before updating (if any)
  2. Restore stashed changes after updating

WARNING: Without stash enabled, any uncommitted changes (staged or unstaged)
will be destroyed during the update process. Use --stash or set git.update.stash
in your config to preserve your work.

Pull Strategies (--pull-strategy):
  default  Use each repository's own pull configuration (default)
  ff-only  Fail rather than create a merge commit
  rebase   Rebase local commits onto the upstream branch
  merge    Always merge when histories have diverged

The discard path can be tuned with two config keys: git.update.clean-ignored
also removes ignored files, and git.update.submodules controls whether
submodules are re-initialized.

The default branch name is determined from the repository catalog
configuration, which typically reads it from the git repository's
HEAD reference or uses a configured default.`,
		Example: `  # Update specific repositories
  batch-tool git update repo1 repo2

  # Update all repositories
  batch-tool git update ~all

  # Update with automatic stash/restore of uncommitted changes
  batch-tool git update --stash repo1 repo2

  # Refuse to update when a merge would be required
  batch-tool git update --pull-strategy ff-only ~backend`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			config.Viper(cmd.Context()).BindPFlag(config.GitUpdatePullStrategy, cmd.Flags().Lookup(pullStrategyFlag))

			if err := utils.ValidateEnumConfig(cmd, config.GitUpdatePullStrategy, AvailablePullStrategies); err != nil {
				return err
			}

			return utils.BindBoolFlags(cmd, config.GitUpdateStash, stashFlag, noStashFlag)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.Viper(cmd.Context()).GetBool(config.GitUpdateStash) {
				return call.Do(cmd, args, call.Wrap(StashPush, Update, StashPop))
			}

			return call.Do(cmd, args, call.Wrap(Clean, Update))
		},
	}

	// Default false to match git.update.stash, so --help does not advertise the wrong default.
	utils.BuildBoolFlagsDefault(updateCmd, stashFlag, "", noStashFlag, "", false, "Automatically stash and restore uncommitted changes during update")
	updateCmd.Flags().String(pullStrategyFlag, PullDefault, "how to reconcile diverged history: \"default\", \"ff-only\", \"rebase\", or \"merge\"")

	return updateCmd
}

// Update checks out the default branch and pulls the latest changes.
func Update(ctx context.Context, ch output.Channel) error {
	branch := catalog.GetBranchForRepo(ctx, ch.Name())

	pullArgs := []string{"pull"}
	switch config.Viper(ctx).GetString(config.GitUpdatePullStrategy) {
	case PullFFOnly:
		pullArgs = append(pullArgs, "--ff-only")
	case PullRebase:
		pullArgs = append(pullArgs, "--rebase")
	case PullMerge:
		pullArgs = append(pullArgs, "--no-rebase")
	case PullDefault:
		// defer to the repository's own pull configuration
	}

	return call.Wrap(
		call.Exec("git", "checkout", branch),
		call.Exec("git", pullArgs...),
	)(ctx, ch)
}

// Clean resets any uncommitted changes, removes untracked files, and re-initializes submodules.
// Removal of ignored files and submodule handling are both configurable.
func Clean(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	cleanArgs := []string{"clean", "-fd"}
	if viper.GetBool(config.GitUpdateCleanIgnored) {
		cleanArgs = append(cleanArgs, "-x")
	}

	steps := []call.Func{
		call.Exec("git", "reset", "--hard"),
		call.Exec("git", cleanArgs...),
	}

	if viper.GetBool(config.GitUpdateSubmodules) {
		steps = append(steps,
			call.Exec("git", "submodule", "deinit", "-f", "."),
			call.Exec("git", "submodule", "update", "--init", "--recursive"),
		)
	}

	return call.Wrap(steps...)(ctx, ch)
}
