package git

import (
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

func addUpdateCmd() *cobra.Command {
	// updateCmd represents the update command
	updateCmd := &cobra.Command{
		Use:   "update <repository> ...",
		Short: "Update primary branch across repositories",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(gitUpdate))
		},
	}

	return updateCmd
}

func gitUpdate(repo string, ch chan<- string) error {
	cmd := exec.Command("git", "checkout", viper.GetString(config.SourceBranch))
	cmd.Dir = utils.RepoPath(repo)

	_, err := cmd.Output()
	if err != nil {
		return err
	}

	cmd = exec.Command("git", "pull")
	cmd.Dir = utils.RepoPath(repo)

	output, err := cmd.Output()
	if err != nil {
		return err
	}

	ch <- string(output)

	return nil
}
