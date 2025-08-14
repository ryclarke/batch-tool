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
			if label == viper.GetString(config.SuperSetLabel) {
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
	forcedSet := mapset.NewSet[string]()

	// Exclude unwanted labels by default
	if viper.GetBool(config.SkipUnwanted) {
		for _, unwanted := range viper.GetStringSlice(config.UnwantedLabels) {
			filters = append(filters, unwanted+viper.GetString(config.TokenLabel)+viper.GetString(config.TokenSkip))
		}
	}

	for _, filter := range filters {
		// standardize formatting of provided labels
		replacer := strings.NewReplacer(
			viper.GetString(config.TokenLabel), "",
			viper.GetString(config.TokenSkip), "",
			viper.GetString(config.TokenForced), "",
		)
		filterName := replacer.Replace(filter)

		if strings.Contains(filter, viper.GetString(config.TokenLabel)) {
			filterName = viper.GetString(config.TokenLabel) + filterName
		}

		if strings.Contains(filter, viper.GetString(config.TokenForced)) {
			forcedSet.Add(filterName)
		} else if strings.Contains(filter, viper.GetString(config.TokenSkip)) {
			excludeSet.Add(filterName)
		} else {
			includeSet.Add(filterName)
		}
	}

	includes, excludes, forced := includeSet.ToSlice(), excludeSet.ToSlice(), forcedSet.ToSlice()

	sort.Strings(includes)
	sort.Strings(excludes)
	sort.Strings(forced)

	repoList := RepositoryList(filters...).ToSlice()
	if viper.GetBool(config.SortRepos) {
		sort.Strings(repoList)
	}

	var output strings.Builder

	if len(forced) > 0 {
		_, _ = output.WriteString(fmt.Sprintf("(%s) \u222A ", strings.Join(forced, " \u222A ")))

		if len(excludes) > 0 {
			_, _ = output.WriteString("( ")
		}
	}

	output.WriteString(fmt.Sprintf("(%s)", strings.Join(includes, " \u222A ")))

	if len(excludes) > 0 {
		output.WriteString(fmt.Sprintf(" \u2216 (%s)", strings.Join(excludes, " \u222A ")))

		if len(forced) > 0 {
			_, _ = output.WriteString(" )")
		}
	}

	fmt.Printf("You've selected the following set:\n%s\n\n", output.String())

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
		labelForced := make([]string, 0, len(forced))
		for _, force := range forced {
			if strings.Contains(force, viper.GetString(config.TokenLabel)) {
				labelForced = append(labelForced, strings.ReplaceAll(force, viper.GetString(config.TokenLabel), ""))
			}
		}

		if len(labelForced) > 0 {
			fmt.Printf("\nForced labels:\n")
			PrintLabels(labelForced...)
		}

		labelIncludes := make([]string, 0, len(includes))
		for _, include := range includes {
			if strings.Contains(include, viper.GetString(config.TokenLabel)) {
				labelIncludes = append(labelIncludes, strings.ReplaceAll(include, viper.GetString(config.TokenLabel), ""))
			}
		}

		if len(labelIncludes) > 0 {
			fmt.Printf("\nIncluded labels:\n")
			PrintLabels(labelIncludes...)
		}

		labelExcludes := make([]string, 0, len(excludes))
		for _, exclude := range excludes {
			if strings.Contains(exclude, viper.GetString(config.TokenLabel)) {
				labelExcludes = append(labelExcludes, strings.ReplaceAll(exclude, viper.GetString(config.TokenLabel), ""))
			}
		}

		if len(labelExcludes) > 0 {
			fmt.Printf("\nExcluded labels:\n")
			PrintLabels(labelExcludes...)
		}
	}
}
