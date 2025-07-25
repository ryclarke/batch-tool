package catalog

import (
	"fmt"
	"sort"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
)

// PrintLabels prints the given labels and their matched repositories. If no labels
// are provided, print all available labels (except the superset label).
func PrintLabels(labels ...string) {
	if len(labels) == 0 {
		for label := range Labels {
			if label == supersetLabel {
				continue
			}

			labels = append(labels, label)
		}
	}

	sort.Strings(labels)

	for _, label := range labels {
		if set, ok := Labels[label]; ok && set.Cardinality() > 0 {
			repos := set.ToSlice()
			if viper.GetBool(config.SortRepos) {
				sort.Strings(repos)
			}

			fmt.Printf("  ~ %s ~\n%s\n", label, strings.Join(repos, ", "))
		} else {
			fmt.Printf("  ~ %s ~ (empty label)\n", label)
		}
	}
}

// PrintSet prints a set-theory representation of the provided filters.
func PrintSet(verbose bool, filters ...string) {
	includeSet := mapset.NewSet[string]()
	excludeSet := mapset.NewSet[string]()

	// Exclude unwanted labels by default
	if viper.GetBool(config.SkipUnwanted) {
		for _, unwanted := range viper.GetStringSlice(config.UnwantedLabels) {
			filters = append(filters, unwanted+labelKey+excludeKey)
		}
	}

	for _, filter := range filters {
		// standardize formatting of provided labels
		filterName := strings.ReplaceAll(strings.ReplaceAll(filter, labelKey, ""), excludeKey, "")
		if strings.Contains(filter, labelKey) {
			filterName = labelKey + filterName
		}

		if strings.Contains(filter, excludeKey) {
			excludeSet.Add(filterName)
		} else {
			includeSet.Add(filterName)
		}
	}

	includes, excludes := includeSet.ToSlice(), excludeSet.ToSlice()

	sort.Strings(includes)
	sort.Strings(excludes)

	repoList := RepositoryList(filters...).ToSlice()
	if viper.GetBool(config.SortRepos) {
		sort.Strings(repoList)
	}

	output := fmt.Sprintf("(%s)", strings.Join(includes, " \u222A "))
	if len(excludes) > 0 {
		output += fmt.Sprintf(" \u2216 (%s)", strings.Join(excludes, " \u222A "))
	}

	fmt.Printf("You've selected the following set:\n%s\n\n", output)

	switch n := len(repoList); n {
	case 0:
		fmt.Println("This matches no known repositories")
	case 1:
		fmt.Printf("This matches 1 repository: %s\n", repoList[0])
	default:
		fmt.Printf("This matches %d repositories, listed below:\n%s\n", n, strings.Join(repoList, ", "))
	}

	// print list of repos for each applied label
	if verbose {
		labelIncludes := make([]string, 0, len(includes))
		for _, include := range includes {
			if strings.Contains(include, labelKey) {
				labelIncludes = append(labelIncludes, strings.ReplaceAll(include, labelKey, ""))
			}
		}

		if len(labelIncludes) > 0 {
			fmt.Printf("\nIncluded labels:\n")
			PrintLabels(labelIncludes...)
		}

		labelExcludes := make([]string, 0, len(excludes))
		for _, exclude := range excludes {
			if strings.Contains(exclude, labelKey) {
				labelExcludes = append(labelExcludes, strings.ReplaceAll(exclude, labelKey, ""))
			}
		}

		if len(labelExcludes) > 0 {
			fmt.Printf("\nExcluded labels:\n")
			PrintLabels(labelExcludes...)
		}
	}
}
