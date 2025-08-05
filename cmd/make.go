package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
)

var (
	makeTargets []string
)

func addMakeCmd() *cobra.Command {
	// makeCmd represents the make command
	makeCmd := &cobra.Command{
		Use:   "make <repository> ...",
		Short: "Execute make targets across repositories",
		Long: `Execute make targets across repositories

The provided make targets will be called for each provided repository. Note that some
make targets currently MUST be run synchronously using the '--sync' command line flag.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, repos []string) {
			call.Do(repos, cmd.OutOrStdout(), call.Wrap(call.Exec("make", makeTargets...)))
		},
	}

	makeCmd.Flags().StringSliceVarP(&makeTargets, "target", "t", []string{"format"}, "make target(s)")

	return makeCmd
}
