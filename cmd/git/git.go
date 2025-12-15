package git

import (
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
