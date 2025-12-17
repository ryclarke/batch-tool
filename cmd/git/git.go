package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
	"github.com/spf13/cobra"
)

// Cmd configures the root git command along with all subcommands and flags
func Cmd() *cobra.Command {
	gitCmd := &cobra.Command{
		Use:   "git [cmd] <repository>...",
		Short: "Manage git branches and commits",
		Args:  cobra.MinimumNArgs(1),
	}

	defaultCmd := addStatusCmd()
	gitCmd.Run = defaultCmd.Run
	gitCmd.AddCommand(
		defaultCmd,
		addBranchCmd(),
		addCommitCmd(),
		addDiffCmd(),
		addPushCmd(),
		addUpdateCmd(),
	)

	return gitCmd
}

// ValidateBranch returns an error if the current git branch is the source branch
func ValidateBranch(ctx context.Context, ch output.Channel) error {
	viper := config.Viper(ctx)

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = utils.RepoPath(ctx, ch.Name())
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(output)) == strings.TrimSpace(viper.GetString(config.SourceBranch)) {
		return fmt.Errorf("skipping operation - %s is the source branch", output)
	}

	return nil
}
