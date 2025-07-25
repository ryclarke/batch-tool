package git

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
)

func addDiffCmd() *cobra.Command {
	// diffCmd represents the diff command
	diffCmd := &cobra.Command{
		Use:   "diff <repository> ...",
		Short: "Git diff of each repository",
		Args:  cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			call.Do(args, call.Wrap(call.Exec("git", "diff")))
		},
	}

	return diffCmd
}
