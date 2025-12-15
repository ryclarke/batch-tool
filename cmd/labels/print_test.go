package labels

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

// captureStdout captures stdout during test execution
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

// checkStringEqual verifies two strings are equal
func checkStringEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// checkOutputContains verifies output contains expected strings
func checkOutputContains(t *testing.T, output string, wantStrings []string) {
	t.Helper()
	for _, want := range wantStrings {
		if !strings.Contains(output, want) {
			t.Errorf("Expected output to contain %q, got:\n%s", want, output)
		}
	}
}

// checkOutputNotContains verifies output does not contain unwanted strings
func checkOutputNotContains(t *testing.T, output string, unwantedStrings []string) {
	t.Helper()
	for _, unwanted := range unwantedStrings {
		if strings.Contains(output, unwanted) {
			t.Errorf("Expected output to not contain %q, got:\n%s", unwanted, output)
		}
	}
}

func TestPrintLabels(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name            string
		labels          []string
		wantContains    []string
		wantNotContains []string
		setupLabels     map[string]mapset.Set[string]
		sortRepos       bool
		superSetLabel   string
	}{
		{
			name:   "print all labels when none specified",
			labels: []string{},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
				"backend":  mapset.NewSet("api-server"),
			},
			wantContains: []string{"frontend", "backend", "web-app", "mobile-app", "api-server"},
		},
		{
			name:   "print specific label",
			labels: []string{"frontend"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
				"backend":  mapset.NewSet("api-server"),
			},
			wantContains:    []string{"frontend", "web-app", "mobile-app"},
			wantNotContains: []string{"backend", "api-server"},
		},
		{
			name:   "print empty label",
			labels: []string{"empty"},
			setupLabels: map[string]mapset.Set[string]{
				"empty": mapset.NewSet[string](),
			},
			wantContains: []string{"empty", "empty label"},
		},
		{
			name:   "print non-existent label",
			labels: []string{"nonexistent"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app"),
			},
			wantContains: []string{"nonexistent", "empty label"},
		},
		{
			name:   "skip superset label when printing all",
			labels: []string{},
			setupLabels: map[string]mapset.Set[string]{
				"all":      mapset.NewSet("repo-1", "repo-2"),
				"frontend": mapset.NewSet("repo-1"),
			},
			superSetLabel:   "all",
			wantContains:    []string{"frontend", "repo-1"},
			wantNotContains: []string{"~ all ~"},
		},
		{
			name:   "sorted repos in output",
			labels: []string{"frontend"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("zebra-app", "apple-app", "mobile-app"),
			},
			sortRepos:    true,
			wantContains: []string{"apple-app", "mobile-app", "zebra-app"},
		},
		{
			name:   "multiple labels in sorted order",
			labels: []string{"zeta", "alpha", "beta"},
			setupLabels: map[string]mapset.Set[string]{
				"alpha": mapset.NewSet("app-1"),
				"beta":  mapset.NewSet("app-2"),
				"zeta":  mapset.NewSet("app-3"),
			},
			wantContains: []string{"~ alpha ~", "~ beta ~", "~ zeta ~"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper := config.Viper(ctx)
			viper.Set(config.SortRepos, tt.sortRepos)
			if tt.superSetLabel != "" {
				viper.Set(config.SuperSetLabel, tt.superSetLabel)
			}

			catalog.Labels = tt.setupLabels

			output := captureStdout(t, func() {
				PrintLabels(ctx, tt.labels...)
			})

			checkOutputContains(t, output, tt.wantContains)
			checkOutputNotContains(t, output, tt.wantNotContains)
		})
	}
}

func TestPrintSet(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name            string
		verbose         bool
		filters         []string
		wantContains    []string
		wantNotContains []string
		setupLabels     map[string]mapset.Set[string]
		setupCatalog    map[string]scm.Repository
		skipUnwanted    bool
		unwantedLabels  []string
	}{
		{
			name:    "basic include filter non-verbose",
			verbose: false,
			filters: []string{"~frontend"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"mobile-app": {Name: "mobile-app"},
			},
			wantContains:    []string{"You've selected the following set:", "~frontend", "web-app", "mobile-app", "This matches 2 repositories"},
			wantNotContains: []string{"Included labels:"},
		},
		{
			name:    "basic include filter verbose",
			verbose: true,
			filters: []string{"~frontend"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"mobile-app": {Name: "mobile-app"},
			},
			wantContains: []string{"You've selected the following set:", "~frontend", "Included labels:", "web-app", "mobile-app"},
		},
		{
			name:    "include and exclude filters",
			verbose: false,
			filters: []string{"~all", "!deprecated-app"},
			setupLabels: map[string]mapset.Set[string]{
				"all": mapset.NewSet("web-app", "api-server", "deprecated-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":        {Name: "web-app"},
				"api-server":     {Name: "api-server"},
				"deprecated-app": {Name: "deprecated-app"},
			},
			wantContains:    []string{"~all", "deprecated-app", "∖", "web-app", "api-server"},
			wantNotContains: []string{"This matches 3"},
		},
		{
			name:    "forced inclusion",
			verbose: false,
			filters: []string{"~frontend", "+legacy-app"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"legacy-app": {Name: "legacy-app"},
			},
			wantContains: []string{"∪", "legacy-app", "~frontend", "web-app", "This matches 2 repositories"},
		},
		{
			name:    "forced and excluded with verbose",
			verbose: true,
			filters: []string{"~all", "!~deprecated", "+old-api"},
			setupLabels: map[string]mapset.Set[string]{
				"all":        mapset.NewSet("web-app", "api-server", "old-api"),
				"deprecated": mapset.NewSet("old-api"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"api-server": {Name: "api-server"},
				"old-api":    {Name: "old-api"},
			},
			wantContains: []string{"Excluded labels:", "old-api", "deprecated"},
		},
		{
			name:    "no matches",
			verbose: false,
			filters: []string{"~nonexistent"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app": {Name: "web-app"},
			},
			wantContains: []string{"This matches no known repositories"},
		},
		{
			name:    "single match",
			verbose: false,
			filters: []string{"web-app"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"mobile-app": {Name: "mobile-app"},
			},
			wantContains: []string{"This matches 1 repository: web-app"},
		},
		{
			name:    "skip unwanted labels automatically",
			verbose: false,
			filters: []string{"~all"},
			setupLabels: map[string]mapset.Set[string]{
				"all":        mapset.NewSet("web-app", "deprecated-app"),
				"deprecated": mapset.NewSet("deprecated-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":        {Name: "web-app", Labels: []string{"all"}},
				"deprecated-app": {Name: "deprecated-app", Labels: []string{"all", "deprecated"}},
			},
			skipUnwanted:    true,
			unwantedLabels:  []string{"deprecated"},
			wantContains:    []string{"~all", "web-app"},
			wantNotContains: []string{"deprecated-app"},
		},
		{
			name:    "direct repo names without labels",
			verbose: false,
			filters: []string{"web-app", "api-server"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"api-server": {Name: "api-server"},
			},
			wantContains: []string{"web-app", "api-server", "This matches 2 repositories"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper := config.Viper(ctx)
			viper.Set(config.SkipUnwanted, tt.skipUnwanted)
			if tt.unwantedLabels != nil {
				viper.Set(config.UnwantedLabels, tt.unwantedLabels)
			}

			catalog.Labels = tt.setupLabels
			catalog.Catalog = tt.setupCatalog

			output := captureStdout(t, func() {
				PrintSet(ctx, tt.verbose, tt.filters...)
			})

			checkOutputContains(t, output, tt.wantContains)
			checkOutputNotContains(t, output, tt.wantNotContains)
		})
	}
}
