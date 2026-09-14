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
	stageFlag   = "stage"

	pushFlag   = "push"
	noPushFlag = "no-" + pushFlag
)

// Staging modes for the commit command, mirroring the three behaviors git itself provides.
const (
	// StageNone commits only what is already in the index.
	StageNone = "none"
	// StageTracked stages modifications and deletions to tracked files, like `git commit -a`.
	StageTracked = "tracked"
	// StageAll stages everything including untracked files, like `git add -A`.
	StageAll = "all"
)

// AvailableStageModes lists the valid values for the --stage flag.
var AvailableStageModes = []string{StageNone, StageTracked, StageAll}

func addCommitCmd() *cobra.Command {
	// commitCmd represents the commit command
	commitCmd := &cobra.Command{
		Use:   "commit {-m <message>|--amend [-m <message>]} [-s <mode>] [--push] <repository>...",
		Short: "Commit code changes across repositories",
		Long: `Commit changes across multiple repositories.

Staging Modes (-s|--stage):
  all      Stage every change, including untracked files (default)
  tracked  Stage modifications and deletions to tracked files only
  none     Commit only what is already staged

The default can be set with git.commit.stage in your config file. Pushing is
opt-in via --push (or git.commit.push), since 'git push' is also a standalone
command.

Safety Features:
  - Prevents committing on the default/primary branch (use a feature branch)
  - Requires a commit message (unless using --amend)`,
		Example: `  # Commit with a message
  batch-tool git commit -m "Add user authentication" repo1 repo2

  # Commit only changes that were already staged
  batch-tool git commit -s none -m "Partial fix" repo1

  # Commit tracked modifications without picking up new files
  batch-tool git commit -s tracked -m "Tweak config" ~backend

  # Commit amend (no message) and push
  batch-tool git commit --amend --push repo1`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.GitCommitMessage, cmd.Flags().Lookup(messageFlag))
			viper.BindPFlag(config.GitCommitAmend, cmd.Flags().Lookup(amendFlag))
			viper.BindPFlag(config.GitCommitStage, cmd.Flags().Lookup(stageFlag))

			if err := utils.ValidateEnumConfig(cmd, config.GitCommitStage, AvailableStageModes); err != nil {
				return err
			}

			if err := utils.BindBoolFlags(cmd, config.GitCommitPush, pushFlag, noPushFlag); err != nil {
				return err
			}

			if viper.GetBool(config.GitCommitAmend) {
				if viper.GetBool(config.GitCommitPush) {
					viper.Set(config.GitPushForce, true) // force push is implied when pushing an amended commit
				}

				return nil // amended commits do not require a message
			}

			return utils.ValidateRequiredConfig(cmd.Context(), config.GitCommitMessage)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return call.Do(cmd, args, call.Wrap(ValidateBranch(), Commit))
		},
	}

	commitCmd.Flags().StringP(messageFlag, "m", "", "commit message (required for new commits)")
	commitCmd.Flags().StringP(stageFlag, "s", StageAll, "changes to stage before committing: \"none\", \"tracked\", or \"all\"")

	// No shorthand for --amend: git reads -a as --all, and -p is claimed by the global --print flag.
	commitCmd.Flags().Bool(amendFlag, false, "amend the latest existing commit")
	utils.BuildBoolFlagsDefault(commitCmd, pushFlag, "", noPushFlag, "", false, "push the commit to the remote repository")

	return commitCmd
}

// Commit stages changes according to the configured staging mode, creates a commit,
// and optionally pushes it to the remote repository.
func Commit(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	steps := make([]call.Func, 0, 3)
	commitArgs := []string{"commit"}

	// Stage changes according to the configured mode. StageAll needs a separate
	// `git add` because `git commit -a` never picks up untracked files.
	switch viper.GetString(config.GitCommitStage) {
	case StageAll:
		steps = append(steps, call.Exec("git", "add", "-A"))
	case StageTracked:
		commitArgs = append(commitArgs, "-a")
	case StageNone:
		// commit the index as-is
	}

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

	if viper.GetBool(config.GitCommitPush) {
		steps = append(steps, Push)
	}

	return call.Wrap(steps...)(ctx, ch)
}
