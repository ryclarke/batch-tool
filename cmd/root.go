// Package cmd provides the command-line interface for batch-tool.
package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/cmd/exec"
	"github.com/ryclarke/batch-tool/cmd/git"
	"github.com/ryclarke/batch-tool/cmd/make"
	"github.com/ryclarke/batch-tool/cmd/pr"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"

	// Register SCM providers
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

	catalogFlushFlag = "flush"
)

// RootCmd configures the top-level root command along with all subcommands and flags
func RootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "batch-tool",
		Short: "Batch tool for working across multiple git repositories",
		Long: `Batch tool for working across multiple git repositories

This tool provides a collection of utility functions that facilitate work across
multiple git repositories, including branch management and pull request creation.

Repository Selection:
  Most commands accept repository arguments that can be specified in several ways.

  Repository Names:
    Repositories are tracked with project/name paths (e.g. myproject/repo1).
    The project prefix can be omitted if it matches the configured default project.
    Examples: repo1 repo2 myproject/repo3

  Labels/Aliases (~ prefix):
    Use SCM labels or configured local aliases to select groups of repositories.
    Examples: ~backend ~frontend ~all

    Note: ~all is an implicit alias that includes all tracked repositories.

  Force Include (+ prefix):
    Include repositories or labels that would normally be excluded/unwanted.
    Examples: +excluded-repo +~unwanted-label

  Exclude (! prefix):
    Explicitly exclude specific repositories or labels from selection.
    Examples: !problem-repo !~experimental

  Combining Selectors:
    Mix and match different selection methods in a single command.
    Example: batch-tool git status repo1 ~backend +special !~experimental

Shell Note:
  Special characters (!, +, ~) may need escaping depending on your shell.
  For Bash/Zsh, use quotes or backslashes: '!repo' or \!repo`,
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
			if sync, err := cmd.Flags().GetBool(syncFlag); err == nil && sync {
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
		catalog.Init(ctx, false)
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

	// If printing is requested without explicit wait/no-wait, disable wait by default regardless of terminal state
	if shouldPrint, err := cmd.Flags().GetBool(printFlag); err == nil && shouldPrint {
		viper.Set(config.WaitOnExit, false)
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
		Use:     "labels [repository|label]...",
		Aliases: []string{"label"},
		Short:   "Inspect repository labels and test filters",
		Long: `Inspect repository labels and test filter expressions.

This command displays labels (topics and aliases) for specified repositories,
helping you understand which repositories match your filter expressions. It's
useful for testing repository selection before running commands that modify state.

If run without any filter arguments, the command lists all labels and aliases.

Label Types:
  SCM Topics/Labels:
    Labels assigned to repositories in your source control management system
    (GitHub topics, Bitbucket labels, etc.). These are fetched from the API
    and cached in the repository catalog.

  Local Aliases:
    Custom label aliases defined in your configuration file. These are merged
	with discovered SCM labels if there's a name collision.

Filter Testing:
  Use this command to preview which repositories will be selected by your
  filter expressions (labels, exclusions, force-includes) before executing
  commands that make changes. This helps avoid mistakes when working with
  repository selections.

Verbose Mode:
  The -v/--verbose flag expands label references to show all repositories
  that would be included by each label in your filter expression.`,
		Example: `# Test a label filter
  batch-tool labels ~web ~db

  # Test complex filter with exclusions and verbose output
  batch-tool labels -v ~all !~experimental`,
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
	cmd := &cobra.Command{
		Use:   "catalog [--flush]",
		Short: "Print information on the cached repository catalog",
		Long: `Display the local repository catalog with metadata.

This command shows all repositories tracked by batch-tool along with their
metadata, including project names, default branches, visibility settings,
descriptions, and assigned SCM labels.

Catalog Source:
  The catalog is built by querying your configured SCM provider and is
  cached locally with a configurable TTL. The cache improves performance
  by avoiding repeated API lookup calls.

Catalog Contents:
  For each repository, the catalog displays:
    - Repository name
    - Description
    - Project namespace
    - Default branch name
    - Visibility (public/private)
    - Associated SCM labels

Cache Management:
  The catalog cache is automatically refreshed when it expires (based on TTL).
  Use the -f/--flush flag to force an immediate refresh, which is useful when
  you've made changes to repository metadata in your SCM provider.`,
		Example: `  # Display the catalog
  batch-tool catalog

  # Force refresh the catalog cache
  batch-tool catalog -f`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			if flush, err := cmd.Flags().GetBool(catalogFlushFlag); err == nil && flush {
				// Flush and re-initialize the catalog even if the TTL has not expired
				catalog.Init(cmd.Context(), true)
			}

			output.GetCatalogHandler(cmd.Context())(cmd)
		},
	}

	cmd.Flags().BoolP(catalogFlushFlag, "f", false, "Force refresh of catalog cache")

	return cmd
}
