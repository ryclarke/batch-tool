package make

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
)

const (
	targetFlag = "target"
)

// Cmd configures the make command
func Cmd() *cobra.Command {
	makeCmd := &cobra.Command{
		Use:   "make <repository>...",
		Short: "Execute make targets across repositories",
		Long: `Execute make targets across repositories

The provided make targets will be called for each provided repository. Note that some
make targets currently MUST be run synchronously using the '--sync' command line flag.`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.MakeTargets, cmd.Flags().Lookup(targetFlag))

			return nil
		},
		Run: func(cmd *cobra.Command, repos []string) {
			call.Do(cmd, repos, Make)
		},
	}

	makeCmd.Flags().StringSliceP(targetFlag, "t", []string{"format"}, "make target(s)")

	return makeCmd
}

// Make runs the specified make targets in the given repository.
func Make(ctx context.Context, ch output.Channel) error {
	targets := config.Viper(ctx).GetStringSlice(config.MakeTargets)

	return call.Exec("make", targets...)(ctx, ch)
}
