package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/cisco-batch-tool/call"
)

var (
	makeTargets []string
)

func addMakeCmd() *cobra.Command {
	// makeCmd represents the make command
	makeCmd := &cobra.Command{
		Use:   "make <repository> ...",
		Short: "Execute make across repositories",
		Long: `Execute make across repositories

The provided make targets will be called for each provided repository. Note that some
make targets currently MUST be run synchronously using the '--sync' command line flag.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(_ *cobra.Command, repos []string) {
			call.Do(repos, call.Wrap(call.Exec("make", makeTargets...)))
		},
	}

	makeCmd.Flags().StringSliceVarP(&makeTargets, "target", "t", []string{"format"}, "make target(s)")

	return makeCmd
}
