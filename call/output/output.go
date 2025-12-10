package output

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/cobra"
)

const (
	Native = "native"
	TUI    = "tui"
)

var AvailableHandlers = []string{TUI, Native}

// Handler represents a function for processing streaming command output.
type Handler func(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error)

// GetHandler returns an output Handler based on the configuration.
func GetHandler(ctx context.Context) Handler {
	viper := config.Viper(ctx)
	handlerType := viper.GetString(config.OutputStyle)

	switch handlerType {
	case Native:
		return NativeHandler
	default:
		// Use more advanced Bubbletea handler by default
		return BubbleteaHandler
	}
}

// LabelHandler represents a function for displaying labels.
type LabelHandler func(cmd *cobra.Command, verbose bool, filters ...string)

// GetLabelHandler returns a LabelHandler based on the configuration.
func GetLabelHandler(ctx context.Context) LabelHandler {
	viper := config.Viper(ctx)
	handlerType := viper.GetString(config.OutputStyle)

	switch handlerType {
	case Native:
		return NativeLabels
	default:
		// Use more advanced Bubbletea handler by default
		return BubbleteaLabels
	}
}

// CatalogHandler represents a function for displaying the repository catalog.
type CatalogHandler func(cmd *cobra.Command)

// GetCatalogHandler returns a CatalogHandler based on the configuration.
func GetCatalogHandler(ctx context.Context) CatalogHandler {
	viper := config.Viper(ctx)
	handlerType := viper.GetString(config.OutputStyle)

	switch handlerType {
	case Native:
		return NativeCatalog
	default:
		// Use more advanced Bubbletea handler by default
		return BubbleteaCatalog
	}
}

// NativeHandler is a simple output Handler that batches and prints output from each repository's channels in sequence.
// It is straightforward and compatible with all terminal environments, but lacks interactivity and modern UI features.
func NativeHandler(cmd *cobra.Command, repos []string, output []<-chan string, errs []<-chan error) {
	for i, repo := range repos {
		// print header with repository name
		fmt.Fprintf(cmd.OutOrStdout(), "\n------ %s ------", repo)

		// print all output for this repo to Stdout
		for msg := range output[i] {
			fmt.Fprintln(cmd.OutOrStdout(), msg)
		}

		// print any errors for this repo to Stderr
		for err := range errs[i] {
			fmt.Fprintln(cmd.ErrOrStderr(), "ERROR: ", err)
		}
	}
}

// NativeLabels prints labels in the native/terminal output format.
// When no filters are provided, it prints all available labels and their repositories.
// When filters are provided, it prints a set-theory representation of the filter and matched repos.
func NativeLabels(cmd *cobra.Command, verbose bool, filters ...string) {
	ctx := cmd.Context()
	if len(filters) > 0 {
		labelGroup, repos := catalog.ParseLabels(ctx, filters...)
		printSet(cmd, verbose, labelGroup, repos)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Available labels:")
		printLabels(cmd)
	}
}

// printLabels prints the given labels and their matched repositories. If no labels
// are provided, print all available labels (except the superset label).
func printLabels(cmd *cobra.Command, labels ...string) {
	ctx := cmd.Context()
	viper := config.Viper(ctx)

	if len(labels) == 0 {
		for label := range catalog.Labels {
			if label == viper.GetString(config.SuperSetLabel) {
				continue
			}

			labels = append(labels, label)
		}
	}

	sort.Strings(labels)

	for _, label := range labels {
		if set, ok := catalog.Labels[label]; ok && set.Cardinality() > 0 {
			repos := set.ToSlice()
			if viper.GetBool(config.SortRepos) {
				sort.Strings(repos)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s ~\n%s\n", label, strings.Join(repos, ", "))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s ~ (empty label)\n", label)
		}
	}
}

// printSet prints a set-theory representation of the provided filters in native format.
func printSet(cmd *cobra.Command, verbose bool, labelGroup catalog.LabelGroup, repos []string) {
	fmt.Fprintf(cmd.OutOrStdout(), "You've selected the following set:\n%s\n\n", labelGroup.String())

	switch n := len(repos); n {
	case 0:
		fmt.Fprintln(cmd.OutOrStdout(), "This matches no known repositories")
	case 1:
		fmt.Fprintf(cmd.OutOrStdout(), "This matches 1 repository: %s\n", repos[0])
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "This matches %d repositories, listed below:\n%s\n", n, strings.Join(repos, ", "))
	}

	// print list of repos for each applied label
	if verbose {
		if labelGroup.Forced.Cardinality() > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nForced labels:\n")
			printLabels(cmd, labelGroup.Forced.ToSlice()...)
		}

		if labelGroup.Included.Cardinality() > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nIncluded labels:\n")
			printLabels(cmd, labelGroup.Included.ToSlice()...)
		}

		if labelGroup.Excluded.Cardinality() > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "\nExcluded labels:\n")
			printLabels(cmd, labelGroup.Excluded.ToSlice()...)
		}
	}
}

// NativeCatalog displays the repository catalog in a simple text format.
func NativeCatalog(cmd *cobra.Command) {
	ctx := cmd.Context()
	viper := config.Viper(ctx)

	// Get sorted repository names
	repoNames := make([]string, 0, len(catalog.Catalog))
	for name := range catalog.Catalog {
		repoNames = append(repoNames, name)
	}
	sort.Strings(repoNames)

	fmt.Fprintf(cmd.OutOrStdout(), "Repository Catalog (%d repositories)\n", len(catalog.Catalog))
	fmt.Fprintln(cmd.OutOrStdout())

	for _, name := range repoNames {
		repo := catalog.Catalog[name]

		fmt.Fprintf(cmd.OutOrStdout(), "## %s\n", name)

		if repo.Description != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "   %s\n", repo.Description)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "   Project: %s | Branch: %s | ", repo.Project, repo.DefaultBranch)
		if repo.Public {
			fmt.Fprintf(cmd.OutOrStdout(), "Visibility: public\n")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Visibility: private\n")
		}

		if len(repo.Labels) > 0 {
			labels := repo.Labels
			if viper.GetBool(config.SortRepos) {
				sort.Strings(labels)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "   Labels: %v\n", labels)
		}

		fmt.Fprintln(cmd.OutOrStdout())
	}
}
