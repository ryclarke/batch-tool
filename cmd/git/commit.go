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

	pushFlag   = "push"
	noPushFlag = "no-" + pushFlag
)

func addCommitCmd() *cobra.Command {
	// commitCmd represents the commit command
	commitCmd := &cobra.Command{
		Use:   "commit {-m <message>|-a [-m <message>]} [--push] <repository>...",
		Short: "Commit code changes across repositories",
		Long: `Commit staged changes across multiple repositories.

This command stages all changes and commits them with the specified message.
It optionally also pushes the commit to the upstream remote.

Safety Features:
  - Prevents committing on the default/primary branch (use a feature branch)
  - Requires a commit message (unless using --amend)`,
		Example: `  # Commit with a message
  batch-tool git commit -m "Add user authentication" repo1 repo2

  # Commit amend (no message) and push
  batch-tool git commit --amend --push repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.GitCommitMessage, cmd.Flags().Lookup(messageFlag))
			viper.BindPFlag(config.GitCommitAmend, cmd.Flags().Lookup(amendFlag))

			if err := utils.BindBoolFlags(cmd, config.GitCommitPush, pushFlag, noPushFlag); err != nil {
				return err
			}

			if viper.GetBool(config.GitCommitAmend) {
				return nil // amended commits do not require a message
			}

			return utils.ValidateRequiredConfig(cmd.Context(), config.GitCommitMessage)
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Wrap(ValidateBranch(), Commit))
		},
	}

	commitCmd.Flags().BoolP(amendFlag, "a", false, "amend the latest existing commit")
	commitCmd.Flags().StringP(messageFlag, "m", "", "commit message (required for new commits)")

	utils.BuildBoolFlags(commitCmd, pushFlag, "u", noPushFlag, "", "push the commit to the remote repository")

	return commitCmd
}

// Commit stages all changes, creates a commit, and pushes it to the remote repository.
func Commit(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	// build the sequence of steps to perform the commit
	steps := []call.Func{
		call.Exec("git", "add", "-A"), // stage all changes for commit
	}

	commitArgs := []string{"commit"}

	msg := viper.GetString(config.GitCommitMessage)
	if msg != "" {
		commitArgs = append(commitArgs, "-m", msg)
	}

	if viper.GetBool(config.GitCommitAmend) {
		commitArgs = append(commitArgs, "--amend", "--reset-author")
		if msg == "" {
			commitArgs = append(commitArgs, "--no-edit")
		}
	}

	steps = append(steps, call.Exec("git", commitArgs...)) // create the commit with the specified args

	// skip pushing the commit if --no-push is set or --push is false
	if viper.GetBool(config.GitCommitPush) {
		steps = append(steps, Push)
	}

	return call.Wrap(steps...)(ctx, ch)
}
