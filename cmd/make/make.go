// Package make provides make target execution functionality for batch-tool.
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
		Use:   "make [-t <target>]... <repository>...",
		Short: "Execute make targets across repositories",
		Long: `Execute make targets across multiple repositories.

This command runs specified make targets in each repository that has a Makefile.
Multiple targets can be specified and will be executed together as a single invocation.

Target Execution:
  Targets are executed by running 'make <target1> <target2> ...' in each
  repository's root directory. The command fails if the Makefile doesn't exist
  or if any target fails.

Synchronous vs Concurrent:
  Some make targets (particularly those that modify files or run builds) may
  need to be run synchronously to avoid conflicts. Use the --sync flag to
  execute one repository at a time.`,
		Example: `  # Run default make target
  batch-tool make repo1 repo2

  # Run specific target
  batch-tool make -t test ~backend

  # Run multiple targets together
  batch-tool make -t clean -t build -t test repo1

  # Run targets synchronously (one repo at a time)
  batch-tool make --sync -t build ~all`,
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

	makeCmd.Flags().StringSliceP(targetFlag, "t", nil, "make target(s), can be specified multiple times")

	return makeCmd
}

// Make runs the specified make targets in the given repository.
func Make(ctx context.Context, ch output.Channel) error {
	targets := config.Viper(ctx).GetStringSlice(config.MakeTargets)

	return call.Exec("make", targets...)(ctx, ch)
}
