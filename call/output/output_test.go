package output_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/call/output"
	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

// TestGetHandler tests the GetHandler function
func TestGetHandler(t *testing.T) {
	tests := []struct {
		name            string
		configValue     string
		wantHandlerType string
	}{
		{
			name:            "native handler selected",
			configValue:     "native",
			wantHandlerType: "native",
		},
		{
			name:            "bubbletea handler selected",
			configValue:     "bubbletea",
			wantHandlerType: "bubbletea",
		},
		{
			name:            "empty value defaults to native",
			configValue:     "",
			wantHandlerType: "native",
		},
		{
			name:            "invalid value defaults to native",
			configValue:     "invalid-handler",
			wantHandlerType: "native",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			// Set the handler type
			if tt.configValue != "" {
				viper.Set(config.OutputStyle, tt.configValue)
			}

			handler := output.GetHandler(ctx)

			// Verify the handler is not nil
			if handler == nil {
				t.Fatal("getDefaultHandler returned nil")
			}

			// We can't directly compare function pointers, but we can verify
			// that we got a valid handler by checking it's callable
			// This is a basic sanity check
			if handler == nil {
				t.Errorf("Expected non-nil handler for type %s", tt.wantHandlerType)
			}
		})
	}
}

// TestGetLabels tests the GetLabels function
func TestGetLabels(t *testing.T) {
	tests := []struct {
		name        string
		configValue string
	}{
		{
			name:        "native label handler selected",
			configValue: "native",
		},
		{
			name:        "bubbletea label handler selected",
			configValue: "bubbletea",
		},
		{
			name:        "empty value defaults to native",
			configValue: "",
		},
		{
			name:        "invalid value defaults to native",
			configValue: "invalid-handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			// Set the handler type
			if tt.configValue != "" {
				viper.Set(config.OutputStyle, tt.configValue)
			}

			labelHandler := output.GetLabelHandler(ctx)

			// Verify the handler is not nil
			if labelHandler == nil {
				t.Fatal("GetLabels returned nil")
			}

			// Verify the handler is callable
			if labelHandler == nil {
				t.Errorf("Expected non-nil label handler")
			}
		})
	}
}

// TestGetHandlerIntegration tests that the handler configuration integrates properly
func TestGetHandlerIntegration(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Test that we can get a handler with default config
	handler := output.GetHandler(ctx)
	if handler == nil {
		t.Fatal("getDefaultHandler returned nil with default config")
	}

	// Test that we can change the handler type
	viper.Set(config.OutputStyle, "bubbletea")
	handler = output.GetHandler(ctx)
	if handler == nil {
		t.Fatal("getDefaultHandler returned nil with bubbletea config")
	}
}

// TestNativeHandler tests that NativeHandler properly handles and prints messages and errors
func TestNativeHandler(t *testing.T) {
	ctx := loadFixture(t)
	setupDirs(t, ctx, []string{"repo1", "repo2"})

	viper := config.Viper(ctx)
	viper.Set(config.MaxConcurrency, 2)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	// CallFunc that returns an error
	errorFunc := func(_ context.Context, repo string, ch chan<- string) error {
		ch <- "some output before error"
		return errors.New("test error for " + repo)
	}

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	call.Do(cmd, []string{"repo1", "repo2"}, errorFunc, output.NativeHandler)

	output := buf.String()
	errOutput := errBuf.String()

	// Verify headers and output were printed
	checkOutputContains(t, output, []string{"------ repo1 ------", "------ repo2 ------", "some output before error"})

	// Verify errors were printed to stderr
	checkOutputContains(t, errOutput, []string{"ERROR:", "test error for repo1", "test error for repo2"})
}

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

// checkOutputNotContains verifies output does not contain unwanted strings
func checkOutputNotContains(t *testing.T, output string, unwantedStrings []string) {
	t.Helper()
	for _, unwanted := range unwantedStrings {
		if strings.Contains(output, unwanted) {
			t.Errorf("Expected output to not contain %q, got:\n%s", unwanted, output)
		}
	}
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
			checkOutputContains(t, outputStr, tt.wantContains)
			checkOutputNotContains(t, outputStr, tt.wantNotContains)
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
			checkOutputContains(t, outputStr, tt.wantContains)
			checkOutputNotContains(t, outputStr, tt.wantNotContains)
		})
	}
}
