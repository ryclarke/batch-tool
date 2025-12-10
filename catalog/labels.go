package catalog

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

// set-theory notation symbols
const (
	union = "∪" // U+222A
	minus = "∖" // U+2216
)

// Label represents a logical grouping of repository names.
type Label struct {
	mapset.Set[string]
}

// String provides a set-theory representation of the Label.
func (l *Label) String() string {
	slice := l.ToSlice()
	slices.Sort(slice)

	return strings.Join(slice, " "+union+" ")
}

// LabelGroup holds categorized label names extracted from a list of filter arguments.
type LabelGroup struct {
	Forced   Label
	Included Label
	Excluded Label
}

// String provides a set-theory representation of the LabelGroup.
//
//	(forced ∪ (included ∖ excluded))
func (lg LabelGroup) String() string {
	// Build set notation string
	var setBuilder strings.Builder

	if lg.Forced.Cardinality() > 0 {
		setBuilder.WriteString(fmt.Sprintf("(%s) %s ", lg.Forced.String(), union))
		if lg.Excluded.Cardinality() > 0 {
			setBuilder.WriteString("( ")
		}
	}

	setBuilder.WriteString(fmt.Sprintf("(%s)", lg.Included.String()))

	if lg.Excluded.Cardinality() > 0 {
		setBuilder.WriteString(fmt.Sprintf(" %s (%s)", minus, lg.Excluded.String()))
		if lg.Forced.Cardinality() > 0 {
			setBuilder.WriteString(" )")
		}
	}

	return setBuilder.String()
}

// ToSlices converts the LabelGroup sets to sorted slices.
func (lg LabelGroup) ToSlices() (forced, included, excluded []string) {
	forced = lg.Forced.ToSlice()
	sort.Strings(forced)

	included = lg.Included.ToSlice()
	sort.Strings(included)

	excluded = lg.Excluded.ToSlice()
	sort.Strings(excluded)

	return
}

// ParseLabels constructs a parsed group of labels based on the provided filters, along with the
// list of matching repositories from the local cache.
func ParseLabels(ctx context.Context, filters ...string) (LabelGroup, []string) {
	viper := config.Viper(ctx)

	labels := LabelGroup{
		Forced:   Label{mapset.NewSet[string]()},
		Included: Label{mapset.NewSet[string]()},
		Excluded: Label{mapset.NewSet[string]()},
	}

	// Exclude unwanted labels by default
	if viper.GetBool(config.SkipUnwanted) {
		for _, unwanted := range viper.GetStringSlice(config.UnwantedLabels) {
			filters = append(filters, unwanted+viper.GetString(config.TokenSkip)+viper.GetString(config.TokenLabel))
		}
	}

	for _, filter := range filters {
		// standardize formatting of provided filters
		filterName := cleanName(ctx, filter)

		if strings.Contains(filter, viper.GetString(config.TokenForced)) {
			labels.Forced.Add(filterName)
		} else if strings.Contains(filter, viper.GetString(config.TokenSkip)) {
			labels.Excluded.Add(filterName)
		} else {
			labels.Included.Add(filterName)
		}
	}

	// Get matched repos
	repoList := RepositoryList(ctx, filters...).ToSlice()
	if viper.GetBool(config.SortRepos) {
		sort.Strings(repoList)
	}

	return labels, repoList
}

// clean the filter name and re-append label token if needed
func cleanName(ctx context.Context, filter string) string {
	viper := config.Viper(ctx)
	name := utils.CleanFilter(ctx, filter)

	if strings.Contains(filter, viper.GetString(config.TokenLabel)) {
		name = name + viper.GetString(config.TokenLabel)
	}

	return name
}
