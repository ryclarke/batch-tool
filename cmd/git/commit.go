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
		Use:               "commit <repository>...",
		Short:             "Commit code changes across repositories",
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
			call.Do(cmd, args, call.Wrap(ValidateBranch, Commit))
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
