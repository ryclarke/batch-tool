package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/cmd/exec"
	"github.com/ryclarke/batch-tool/cmd/git"
	"github.com/ryclarke/batch-tool/cmd/labels"
	"github.com/ryclarke/batch-tool/cmd/make"
	"github.com/ryclarke/batch-tool/cmd/pr"
	"github.com/ryclarke/batch-tool/config"

	// Register the SCM providers
	_ "github.com/ryclarke/batch-tool/scm/bitbucket"
	_ "github.com/ryclarke/batch-tool/scm/github"
)

const (
	configFlag         = "config"
	sortFlag           = "sort"
	noSortFlag         = "no-sort"
	syncFlag           = "sync"
	skipUnwantedFlag   = "skip-unwanted"
	noSkipUnwantedFlag = "no-skip-unwanted"
	maxConcurrencyFlag = "max-concurrency"
)

// RootCmd configures the top-level root command along with all subcommands and flags
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "batch-tool",
		Short: "Batch tool for working across multiple git repositories",
		Long: `Batch tool for working across multiple git repositories

This tool provides a collection of utility functions that facilitate work across
multiple git repositories, including branch management and pull request creation.`,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.MaxConcurrency, cmd.PersistentFlags().Lookup(maxConcurrencyFlag))
			viper.BindPFlag(config.SortRepos, cmd.PersistentFlags().Lookup(sortFlag))
			viper.BindPFlag(config.SkipUnwanted, cmd.PersistentFlags().Lookup(skipUnwantedFlag))

			// Allow the `--sync` flag to override max-concurrency to 1
			if sync, _ := cmd.Flags().GetBool(syncFlag); sync {
				viper.Set(config.MaxConcurrency, 1)
			}

			// Allow the `--no-sort` flag to override sorting configuration
			if noSort, _ := cmd.Flags().GetBool(noSortFlag); noSort {
				viper.Set(config.SortRepos, false)
			}

			// Allow the `--no-skip-unwanted` flag to override label skipping configuration
			if noSkip, _ := cmd.Flags().GetBool(noSkipUnwantedFlag); noSkip {
				viper.Set(config.SkipUnwanted, false)
			}
		},
		Version: config.Version,
	}

	// Add all subcommands to the root
	rootCmd.AddCommand(
		&cobra.Command{
			Use:   "catalog",
			Short: "Print information on the cached repository catalog",
			Run: func(_ *cobra.Command, _ []string) {
				fmt.Printf("%v\n", catalog.Catalog)
			},
		},
		exec.Cmd(),
		git.Cmd(),
		labels.Cmd(),
		make.Cmd(),
		pr.Cmd(),
	)

	rootCmd.PersistentFlags().StringVar(&config.CfgFile, configFlag, "", "config file (default is batch-tool.yaml)")

	rootCmd.PersistentFlags().Bool(syncFlag, false, "execute commands synchronously (alias for --max-concurrency=1)")
	rootCmd.PersistentFlags().Int(maxConcurrencyFlag, runtime.NumCPU(), "maximum number of concurrent operations")
	rootCmd.PersistentFlags().Bool(sortFlag, true, "sort the provided repositories")
	rootCmd.PersistentFlags().Bool(skipUnwantedFlag, true, "skip undesired labels (default: deprecated,poc)")

	// --no-sort is excluded from usage and help output, and is an alternative to --sort=false
	rootCmd.PersistentFlags().Bool(noSortFlag, false, "")
	rootCmd.PersistentFlags().MarkHidden(noSortFlag)

	// --no-skip-unwanted is excluded from usage and help output, and is an alternative to --skip-unwanted=false
	rootCmd.PersistentFlags().Bool(noSkipUnwantedFlag, false, "")
	rootCmd.PersistentFlags().MarkHidden(noSkipUnwantedFlag)

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	ctx := config.Init(context.Background())
	cobra.OnInitialize(func() {
		catalog.Init(ctx)
	})

	if err := RootCmd().ExecuteContext(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
