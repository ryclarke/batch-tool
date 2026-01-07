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
	forceFlag = "force"
)

func addPushCmd() *cobra.Command {
	// pushCmd represents the push command
	pushCmd := &cobra.Command{
		Use:   "push [flags] <repository>...",
		Short: "Push committed code changes to remote",
		Long: `Push local commits to the remote repository.

This command pushes the current branch to the remote repository with upstream
tracking enabled (-u flag).

Safety Features:
  - Prevents pushing from the default/primary branch
  - Requires explicit force flag for force pushes
  - Sets upstream tracking automatically

Push Modes:
  Normal:        Push commits to remote (fails if remote has diverged)
  Force (-f):    Force push, overwriting remote history

Force Push Use Cases:
  - After amending commits (rewriting history)
  - After rebasing (rewriting history)
  - When you need to overwrite remote with local state

WARNING: Force push rewrites history on the remote. Use with caution,
especially on shared branches.

Use Cases:
  - Push commits after local development
  - Share feature branches with team
  - Update remote after amending commits
  - Synchronize local and remote state`,
		Example: `  # Push current branch to remote
  batch-tool git push repo1 repo2

  # Push all backend services
  batch-tool git push ~backend

  # Force push after amending commits
  batch-tool git push -f repo1 repo2

  # Push with synchronous execution
  batch-tool git push --sync repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRun: func(cmd *cobra.Command, _ []string) {
			viper := config.Viper(cmd.Context())
			viper.BindPFlag(config.CommitAmend, cmd.Flags().Lookup(forceFlag))
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(ValidateBranch(), Push))
		},
	}

	pushCmd.Flags().BoolP(forceFlag, "f", false, "overwrite remote with local changes")

	return pushCmd
}

// Push committed changes to the remote repository.
func Push(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)
	branch, err := utils.LookupBranch(ctx, ch.Name())
	if err != nil {
		return err
	}

	args := []string{"push", "-u", "origin", branch}
	if viper.GetBool(config.CommitAmend) {
		args = append(args, "-f")
	}

	return call.Exec("git", args...)(ctx, ch)
}
