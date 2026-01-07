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
	messageFlag = "message"
	amendFlag   = "amend"
	pushFlag    = "push"
	noPushFlag  = "no-push"
)

func addCommitCmd() *cobra.Command {
	// commitCmd represents the commit command
	commitCmd := &cobra.Command{
		Use:   "commit -m <message> [flags] <repository>...",
		Short: "Commit code changes across repositories",
		Long: `Commit staged changes across multiple repositories.

This command stages all changes and commits them with the specified message.
It runs 'git add -A' followed by 'git commit' in each repository.

Safety Features:
  - Prevents committing on the default/primary branch (use a feature branch)
  - Requires a commit message (unless using --amend)
  - Optionally pushes changes after commit (configurable)

Commit Modes:
  Normal (-m):   Create a new commit with the specified message
  Amend (--amend): Amend the previous commit (no message required)

Push Behavior:
  By default, commits are pushed to remote after committing (configurable).
  Use --no-push to commit locally without pushing.
  Push happens with -u flag to set upstream tracking.

Use Cases:
  - Commit related changes across multiple services
  - Maintain consistent commit messages
  - Streamline the commit and push workflow
  - Amend previous commits across repositories`,
		Example: `  # Commit with a message
  batch-tool git commit -m "Add user authentication" repo1 repo2

  # Commit all backend services
  batch-tool git commit -m "Fix database query" ~backend

  # Commit without pushing
  batch-tool git commit -m "WIP: refactoring" --no-push repo1

  # Amend the previous commit
  batch-tool git commit --amend repo1 repo2

  # Commit and push with force (for amended commits)
  batch-tool git commit --amend --push repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.CommitMessage, cmd.Flags().Lookup(messageFlag))
			viper.BindPFlag(config.CommitAmend, cmd.Flags().Lookup(amendFlag))
			viper.BindPFlag(config.CommitPush, cmd.Flags().Lookup(pushFlag))

			// Allow the `--no-push` flag to override push configuration
			if noPush, _ := cmd.Flags().GetBool(noPushFlag); noPush {
				viper.Set(config.CommitPush, false)
			}

			if viper.GetBool(config.CommitAmend) {
				return nil // amended commits do not require a message
			}

			return utils.ValidateRequiredConfig(cmd.Context(), config.CommitMessage)
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(ValidateBranch(), Commit))
		},
	}

	commitCmd.Flags().BoolP(amendFlag, "a", false, "amend the latest existing commit")
	commitCmd.Flags().StringP(messageFlag, "m", "", "commit message (required for new commits)")
	commitCmd.Flags().BoolP(pushFlag, "u", true, "push the commit to the remote repository")

	// --no-push is excluded from usage and help output, and is an alternative to --push=false
	commitCmd.PersistentFlags().Bool(noPushFlag, false, "")
	commitCmd.PersistentFlags().MarkHidden(noPushFlag)

	return commitCmd
}

// Commit stages all changes, creates a commit, and pushes it to the remote repository.
func Commit(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	if err := call.Exec("git", "add", ".")(ctx, ch); err != nil {
		return err
	}

	args := []string{"commit"}

	msg := viper.GetString(config.CommitMessage)
	if msg != "" {
		args = append(args, "-m", msg)
	}

	if viper.GetBool(config.CommitAmend) {
		args = append(args, "--amend", "--reset-author")
		if msg == "" {
			args = append(args, "--no-edit")
		}
	}

	if err := call.Exec("git", args...)(ctx, ch); err != nil {
		return err
	}

	// skip pushing the commit if --no-push is set or --push is false
	if !viper.GetBool(config.CommitPush) {
		return nil
	}

	return Push(ctx, ch)
}
