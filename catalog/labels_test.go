package catalog

import (
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/config"
)

func TestLabelString(t *testing.T) {
	tests := []struct {
		name    string
		members []string
		want    string
	}{
		{name: "empty", members: nil, want: ""},
		{name: "single", members: []string{"alpha"}, want: "alpha"},
		{name: "sorted", members: []string{"gamma", "alpha", "beta"}, want: "alpha ∪ beta ∪ gamma"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Label{Set: mapset.NewSet(tt.members...)}
			if got := l.String(); got != tt.want {
				t.Errorf("Label.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLabelGroupString(t *testing.T) {
	tests := []struct {
		name              string
		forced, inc, exc  []string
		wantContainsAll   []string
		wantNotContaining []string
	}{
		{
			name:            "included only",
			inc:             []string{"alpha", "beta"},
			wantContainsAll: []string{"(alpha ∪ beta)"},
		},
		{
			name:            "included and excluded",
			inc:             []string{"alpha"},
			exc:             []string{"omega"},
			wantContainsAll: []string{"(alpha)", "∖", "(omega)"},
		},
		{
			name:            "all three categories",
			forced:          []string{"force1"},
			inc:             []string{"inc1"},
			exc:             []string{"exc1"},
			wantContainsAll: []string{"(force1)", "∪", "(inc1)", "∖", "(exc1)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg := LabelGroup{
				Forced:   Label{Set: mapset.NewSet(tt.forced...)},
				Included: Label{Set: mapset.NewSet(tt.inc...)},
				Excluded: Label{Set: mapset.NewSet(tt.exc...)},
			}

			got := lg.String()
			for _, sub := range tt.wantContainsAll {
				if !strings.Contains(got, sub) {
					t.Errorf("LabelGroup.String() = %q, missing substring %q", got, sub)
				}
			}
		})
	}
}

func TestLabelGroupToSlices(t *testing.T) {
	lg := LabelGroup{
		Forced:   Label{Set: mapset.NewSet("c", "a", "b")},
		Included: Label{Set: mapset.NewSet("z", "y")},
		Excluded: Label{Set: mapset.NewSet("m")},
	}

	forced, inc, exc := lg.ToSlices()

	if !sliceEqual(forced, []string{"a", "b", "c"}) {
		t.Errorf("forced not sorted: %v", forced)
	}
	if !sliceEqual(inc, []string{"y", "z"}) {
		t.Errorf("included not sorted: %v", inc)
	}
	if !sliceEqual(exc, []string{"m"}) {
		t.Errorf("excluded mismatch: %v", exc)
	}
}

func TestParseLabels(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { resetCatalogState(t) })

	v := config.Viper(ctx)
	v.Set(config.SkipUnwanted, false)
	v.Set(config.SortRepos, true)

	tests := []struct {
		name              string
		filters           []string
		wantForcedNames   []string
		wantIncludedNames []string
		wantExcludedNames []string
	}{
		{
			name:              "plain repo",
			filters:           []string{"my-repo"},
			wantIncludedNames: []string{"my-repo"},
		},
		{
			name:              "label is preserved with token",
			filters:           []string{"backend~"},
			wantIncludedNames: []string{"backend~"},
		},
		{
			name:            "force token routes to Forced",
			filters:         []string{"+special"},
			wantForcedNames: []string{"special"},
		},
		{
			name:              "skip token routes to Excluded",
			filters:           []string{"!unwanted"},
			wantExcludedNames: []string{"unwanted"},
		},
		{
			name:              "mixed filters",
			filters:           []string{"keep", "+force", "!drop"},
			wantForcedNames:   []string{"force"},
			wantIncludedNames: []string{"keep"},
			wantExcludedNames: []string{"drop"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lg, _ := ParseLabels(ctx, tt.filters...)

			f, inc, exc := lg.ToSlices()
			if !sliceEqual(f, tt.wantForcedNames) {
				t.Errorf("forced = %v, want %v", f, tt.wantForcedNames)
			}
			if !sliceEqual(inc, tt.wantIncludedNames) {
				t.Errorf("included = %v, want %v", inc, tt.wantIncludedNames)
			}
			if !sliceEqual(exc, tt.wantExcludedNames) {
				t.Errorf("excluded = %v, want %v", exc, tt.wantExcludedNames)
			}
		})
	}
}

func TestParseLabelsAppendsUnwanted(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { resetCatalogState(t) })

	v := config.Viper(ctx)
	v.Set(config.SkipUnwanted, true)
	v.Set(config.UnwantedLabels, []string{"deprecated"})
	v.Set(config.SortRepos, true)

	lg, _ := ParseLabels(ctx, "keep")
	_, _, exc := lg.ToSlices()

	found := false
	for _, e := range exc {
		if strings.Contains(e, "deprecated") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unwanted 'deprecated' label appended to excluded set, got %v", exc)
	}
}

func TestCleanNameReappendsLabelToken(t *testing.T) {
	ctx := loadFixture(t)
	v := config.Viper(ctx)
	token := v.GetString(config.TokenLabel)

	if got := cleanName(ctx, "backend"+token); !strings.HasSuffix(got, token) {
		t.Errorf("cleanName(%q) = %q, expected suffix %q", "backend"+token, got, token)
	}

	if got := cleanName(ctx, "plain-repo"); strings.HasSuffix(got, token) {
		t.Errorf("cleanName(%q) = %q, did not expect label token suffix", "plain-repo", got)
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
