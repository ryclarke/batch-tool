package git

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
)

func addDiffCmd() *cobra.Command {
	// diffCmd represents the diff command
	diffCmd := &cobra.Command{
		Use:               "diff <repository>...",
		Short:             "Git diff of each repository",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		Run: func(cmd *cobra.Command, args []string) {
			call.Do(cmd, args, call.Exec("git", "diff"))
		},
	}

	return diffCmd
}
