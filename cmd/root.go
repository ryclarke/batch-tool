package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ryclarke/batch-tool/call/output"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/cmd/exec"
	"github.com/ryclarke/batch-tool/cmd/git"
	"github.com/ryclarke/batch-tool/cmd/make"
	"github.com/ryclarke/batch-tool/cmd/pr"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"

	// Register the SCM providers
	_ "github.com/ryclarke/batch-tool/scm/bitbucket"
	_ "github.com/ryclarke/batch-tool/scm/github"
)

const (
	configFlag = "config"
	styleFlag  = "style"
	printFlag  = "print"

	waitFlag   = "wait"
	noWaitFlag = "no-" + waitFlag

	skipUnwantedFlag   = "skip-unwanted"
	noSkipUnwantedFlag = "no-" + skipUnwantedFlag

	sortFlag   = "sort"
	noSortFlag = "no-" + sortFlag

	maxConcurrencyFlag = "max-concurrency"
	syncFlag           = "sync"
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

			viper.BindPFlag(config.OutputStyle, cmd.Flags().Lookup(styleFlag))
			viper.BindPFlag(config.PrintResults, cmd.Flags().Lookup(printFlag))
			viper.BindPFlag(config.MaxConcurrency, cmd.Flags().Lookup(maxConcurrencyFlag))

			// Validate output style is a valid selection
			if err := utils.ValidateEnumConfig(cmd, config.OutputStyle, output.AvailableStyles); err != nil {
				return err
			}

			// Don't allow both --max-concurrency and --sync to be set together
			if err := utils.CheckMutuallyExclusiveFlags(cmd, maxConcurrencyFlag, syncFlag); err != nil {
				return err
			}

			// Allow the `--sync` flag to override max-concurrency to 1
			if sync, _ := cmd.Flags().GetBool(syncFlag); sync {
				viper.Set(config.MaxConcurrency, 1)
			}

			if err := utils.BindBoolFlags(cmd, config.SkipUnwanted, skipUnwantedFlag, noSkipUnwantedFlag); err != nil {
				return err
			}

			if err := utils.BindBoolFlags(cmd, config.SortRepos, sortFlag, noSortFlag); err != nil {
				return err
			}

			// Handle wait/no-wait flags with auto-detection for non-interactive environments
			return setTerminalWait(cmd)
		},
		Args:    cobra.NoArgs,
		Version: config.Version,
	}

	// Add all subcommands to the root
	rootCmd.AddCommand(
		catalogCmd(),
		labelsCmd(),
		exec.Cmd(),
		git.Cmd(),
		make.Cmd(),
		pr.Cmd(),
	)

	rootCmd.PersistentFlags().StringVar(&config.CfgFile, configFlag, "", "config file (default is batch-tool.yaml)")
	rootCmd.PersistentFlags().StringP(styleFlag, "o", output.TUI, fmt.Sprintf("output style: \"%v\"", strings.Join(output.AvailableStyles, "\", \"")))
	rootCmd.PersistentFlags().BoolP(printFlag, "p", false, "print results to stdout after processing is complete")
	rootCmd.PersistentFlags().Int(maxConcurrencyFlag, runtime.NumCPU(), "maximum number of concurrent operations")
	rootCmd.PersistentFlags().Bool(syncFlag, false, "execute commands synchronously (same as --max-concurrency=1)")

	utils.BuildBoolFlags(rootCmd, waitFlag, "", noWaitFlag, "q", "wait for user to exit after processing is complete")
	utils.BuildBoolFlags(rootCmd, skipUnwantedFlag, "", noSkipUnwantedFlag, "", "skip configured undesired labels")
	utils.BuildBoolFlags(rootCmd, sortFlag, "", noSortFlag, "", "sort the provided repositories")

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

// setTerminalWait handles auto-detection for non-interactive environments.
func setTerminalWait(cmd *cobra.Command) error {
	viper := config.Viper(cmd.Context())

	// Bind the wait/no-wait flag pair configuration
	if err := utils.BindBoolFlags(cmd, config.WaitOnExit, waitFlag, noWaitFlag); err != nil {
		return err
	}

	// Explicit --wait or --no-wait takes precedence over auto-detection
	if cmd.Flags().Changed(waitFlag) || cmd.Flags().Changed(noWaitFlag) {
		return nil
	}

	// Auto-detect environment type if neither flag is explicitly set
	// This prevents hanging in pipes, redirects, and CI/CD environments
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		viper.Set(config.WaitOnExit, false)
	}

	return nil
}

// labelsCmd configures the labels command
func labelsCmd() *cobra.Command {
	labelsCmd := &cobra.Command{
		Use:               "labels <repository|label>...",
		Aliases:           []string{"label"},
		Short:             "Inspect repository labels and test filters",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: catalog.CompletionFunc(),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Import command(s) from the CLI flag
			verbose, err := cmd.Flags().GetBool("verbose")
			if err != nil {
				return err
			}

			// Get the appropriate label handler based on configured output style
			ctx := cmd.Context()
			output.GetLabelHandler(ctx)(cmd, verbose, args...)

			return nil
		},
	}

	labelsCmd.Flags().BoolP("verbose", "v", false, "expand labels referenced in the given filter")

	return labelsCmd
}

// catalogCmd configures the catalog command
func catalogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "Print information on the cached repository catalog",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			output.GetCatalogHandler(cmd.Context())(cmd)
		},
	}
}
