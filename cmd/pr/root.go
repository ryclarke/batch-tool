package pr

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/utils"
)

var (
	prTitle       string
	prDescription string
)

// Cmd configures the root pr command along with all subcommands and flags
func Cmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "pr [cmd] <repository> ...",
		Short: "Manage pull requests using supported SCM provider APIs",
		Args:  cobra.MinimumNArgs(1),
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return utils.ValidateRequiredConfig(config.AuthToken)
		},
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(args, cmd.OutOrStdout(), call.Wrap(utils.ValidateBranch, getPRCmd))
		},
	}

	rootCmd.PersistentFlags().StringVarP(&prTitle, "title", "t", "", "pull request title")
	rootCmd.PersistentFlags().StringVarP(&prDescription, "description", "d", "", "pull request description")

	rootCmd.PersistentFlags().StringSliceP("reviewer", "r", nil, "pull request reviewer (cecid)")
	viper.BindPFlag(config.Reviewers, rootCmd.PersistentFlags().Lookup("reviewer"))

	rootCmd.AddCommand(
		addNewCmd(),
		addEditCmd(),
		addMergeCmd(),
	)

	return rootCmd
}

func getPRCmd(name string, ch chan<- string) error {
	branch, err := utils.LookupBranch(name)
	if err != nil {
		return fmt.Errorf("failed to lookup branch for %s: %w", name, err)
	}

	provider := scm.Get(viper.GetString(config.GitProvider), viper.GetString(config.GitProject))

	pr, err := provider.GetPullRequest(name, branch)
	if err != nil {
		return fmt.Errorf("failed to get pull request for %s: %w", name, err)
	}

	ch <- fmt.Sprintf("(PR #%d) %s %v\n", pr["id"].(int), pr["title"].(string), pr["reviewers"].([]string))
	if pr["description"].(string) != "" {
		ch <- fmt.Sprintln(pr["description"].(string))
	}

	return nil
}
