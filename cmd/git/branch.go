package git

import (
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

func addBranchCmd() *cobra.Command {
	// branchCmd represents the branch command
	branchCmd := &cobra.Command{
		Use:     "checkout <repository> ...",
		Aliases: []string{"branch"},
		Short:   "Checkout a new branch across repositories",
		Args:    cobra.MinimumNArgs(1),
		PreRunE: func(_ *cobra.Command, _ []string) error {
			return utils.ValidateRequiredConfig(config.Branch)
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(gitUpdate, gitCheckout))
		},
	}

	branchCmd.Flags().StringP("branch", "b", "", "branch name (required)")
	viper.BindPFlag(config.Branch, branchCmd.Flags().Lookup("branch"))

	return branchCmd
}

func gitCheckout(name string, ch chan<- string) error {
	branch := viper.GetString(config.Branch)

	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = utils.RepoPath(name)

	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("git", "checkout", "-B", branch)
		cmd.Dir = utils.RepoPath(name)

		output, err = cmd.Output()
		if err != nil {
			return err
		}

		ch <- string(output)
	}

	ch <- string(output)

	return nil
}
