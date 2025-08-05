package git

import (
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

func addCommitCmd() *cobra.Command {
	// commitCmd represents the commit command
	commitCmd := &cobra.Command{
		Use:   "commit <repository> ...",
		Short: "Commit code changes across repositories",
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if viper.GetBool(config.CommitAmend) {
				return nil // amended commits do not require a message
			}

			return utils.ValidateRequiredConfig(config.CommitMessage)
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(utils.ValidateBranch, gitCommit))
		},
	}

	commitCmd.Flags().BoolP("amend", "a", false, "amend the latest existing commit")
	viper.BindPFlag(config.CommitAmend, commitCmd.Flags().Lookup("amend"))

	commitCmd.Flags().StringP("message", "m", "", "commit message (required for new commits)")
	viper.BindPFlag(config.CommitMessage, commitCmd.Flags().Lookup("message"))

	return commitCmd
}

func gitCommit(name string, ch chan<- string) error {
	branch, err := utils.LookupBranch(name)
	if err != nil {
		return err
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = utils.RepoPath(name)

	if _, err = cmd.Output(); err != nil {
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

	cmd = exec.Command("git", args...)
	cmd.Dir = utils.RepoPath(name)

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	ch <- string(output)

	args = []string{"push", "-u", "origin", branch}
	if viper.GetBool(config.CommitAmend) {
		args = append(args, "-f")
	}

	cmd = exec.Command("git", args...)
	cmd.Dir = utils.RepoPath(name)

	output, err = cmd.Output()
	if err != nil {
		return err
	}

	ch <- string(output)

	return nil
}
