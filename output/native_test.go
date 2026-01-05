package output_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/scm"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

// TestNativeHandler tests that NativeHandler properly handles and prints messages and errors
func TestNativeHandler(t *testing.T) {
	ctx := loadFixture(t)
	testhelper.SetupDirs(t, ctx, []string{"repo1", "repo2"})

	viper := config.Viper(ctx)
	viper.Set(config.MaxConcurrency, 2)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	// Func that returns an error
	errorFunc := func(_ context.Context, ch output.Channel) error {
		ch.WriteString("some output before error")
		return errors.New("test error for " + ch.Name())
	}

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	call.Do(cmd, []string{"repo1", "repo2"}, errorFunc, output.NativeHandler)

	output := buf.String()
	errOutput := errBuf.String()

	// Verify headers and output were printed
	testhelper.AssertContains(t, output, []string{"------ repo1 ------", "------ repo2 ------", "some output before error"})

	// Verify errors were printed to stderr
	testhelper.AssertContains(t, errOutput, []string{"ERROR:", "test error for repo1", "test error for repo2"})
}

func TestNativeLabels_PrintAllLabels(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name            string
		wantContains    []string
		wantNotContains []string
		setupLabels     map[string]mapset.Set[string]
		sortRepos       bool
		superSetLabel   string
	}{
		{
			name: "print all labels when none specified",
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
				"backend":  mapset.NewSet("api-server"),
			},
			wantContains: []string{"Available labels:", "frontend", "backend", "web-app", "mobile-app", "api-server"},
		},
		{
			name: "skip superset label when printing all",
			setupLabels: map[string]mapset.Set[string]{
				"all":      mapset.NewSet("repo-1", "repo-2"),
				"frontend": mapset.NewSet("repo-1"),
			},
			superSetLabel:   "all",
			wantContains:    []string{"Available labels:", "frontend", "repo-1"},
			wantNotContains: []string{"~ all ~"},
		},
		{
			name: "sorted repos in output",
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("zebra-app", "apple-app", "mobile-app"),
			},
			sortRepos:    true,
			wantContains: []string{"Available labels:", "apple-app", "mobile-app", "zebra-app"},
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

			var buf bytes.Buffer
			cmd := fakeCmd(t, ctx, &buf)
			output.NativeLabels(cmd, false)

			outputStr := buf.String()
			testhelper.AssertContains(t, outputStr, tt.wantContains)
			testhelper.AssertNotContains(t, outputStr, tt.wantNotContains)
		})
	}
}

func TestNativeLabels_PrintSet(t *testing.T) {
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
			filters: []string{"frontend~"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"mobile-app": {Name: "mobile-app"},
			},
			wantContains:    []string{"You've selected the following set:", "frontend~", "web-app", "mobile-app", "This matches 2 repositories"},
			wantNotContains: []string{"Included labels:"},
		},
		{
			name:    "basic include filter verbose",
			verbose: true,
			filters: []string{"frontend~"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"mobile-app": {Name: "mobile-app"},
			},
			wantContains: []string{"You've selected the following set:", "frontend~", "Included labels:", "web-app", "mobile-app"},
		},
		{
			name:    "include and exclude filters",
			verbose: false,
			filters: []string{"all~", "!deprecated-app"},
			setupLabels: map[string]mapset.Set[string]{
				"all": mapset.NewSet("web-app", "api-server", "deprecated-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":        {Name: "web-app"},
				"api-server":     {Name: "api-server"},
				"deprecated-app": {Name: "deprecated-app"},
			},
			wantContains:    []string{"all~", "deprecated-app", "∖", "web-app", "api-server"},
			wantNotContains: []string{"This matches 3"},
		},
		{
			name:    "forced inclusion",
			verbose: false,
			filters: []string{"frontend~", "+legacy-app"},
			setupLabels: map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app"),
			},
			setupCatalog: map[string]scm.Repository{
				"web-app":    {Name: "web-app"},
				"legacy-app": {Name: "legacy-app"},
			},
			wantContains: []string{"∪", "legacy-app", "frontend~", "web-app", "This matches 2 repositories"},
		},
		{
			name:    "forced and excluded with verbose",
			verbose: true,
			filters: []string{"all~", "!deprecated~", "+old-api"},
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
			filters: []string{"nonexistent~"},
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
			filters: []string{"all~"},
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
			wantContains:    []string{"all~", "web-app"},
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

			var buf bytes.Buffer
			cmd := fakeCmd(t, ctx, &buf)
			output.NativeLabels(cmd, tt.verbose, tt.filters...)

			outputStr := buf.String()
			testhelper.AssertContains(t, outputStr, tt.wantContains)
			testhelper.AssertNotContains(t, outputStr, tt.wantNotContains)
		})
	}
}
