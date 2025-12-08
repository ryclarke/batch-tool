package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call/output"
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
	outputHandlerFlag  = "style"
)

// RootCmd configures the top-level root command along with all subcommands and flags
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "batch-tool",
		Short: "Batch tool for working across multiple git repositories",
		Long: `Batch tool for working across multiple git repositories

This tool provides a collection of utility functions that facilitate work across
multiple git repositories, including branch management and pull request creation.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			viper := config.Viper(cmd.Context())

			viper.BindPFlag(config.MaxConcurrency, cmd.Flags().Lookup(maxConcurrencyFlag))
			viper.BindPFlag(config.SortRepos, cmd.Flags().Lookup(sortFlag))
			viper.BindPFlag(config.SkipUnwanted, cmd.Flags().Lookup(skipUnwantedFlag))
			viper.BindPFlag(config.OutputStyle, cmd.Flags().Lookup(outputHandlerFlag))

			if outputHandler := viper.GetString(config.OutputStyle); outputHandler != "" && !mapset.NewSet(output.AvailableHandlers...).Contains(outputHandler) {
				return fmt.Errorf("invalid output style: %q (expected one of %v)", viper.GetString(config.OutputStyle), output.AvailableHandlers)
			}

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

			return nil
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
	rootCmd.PersistentFlags().StringP(outputHandlerFlag, "o", output.Native, fmt.Sprintf("output format style: \"%v\"", strings.Join(output.AvailableHandlers, "\", \"")))

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
