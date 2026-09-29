package output_test

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

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

	err := call.Do(cmd, []string{"repo1", "repo2"}, errorFunc, output.NativeHandler)
	if err == nil {
		t.Fatal("Expected Do to return an aggregated failure error")
	}

	output := buf.String()
	errOutput := errBuf.String()

	// Verify headers and output were printed
	testhelper.AssertContains(t, output, []string{"------ repo1 ------", "------ repo2 ------", "some output before error"})

	// Verify errors were printed to stderr
	testhelper.AssertContains(t, errOutput, []string{"ERROR:", "test error for repo1", "test error for repo2"})
}

// TestNativeHandler_LaterRepoFinishesFirst verifies that output stays in repository
// order without deadlocking when a later repository must finish before an earlier
// one can make progress, as happens when it holds the only concurrency slot.
func TestNativeHandler_LaterRepoFinishesFirst(t *testing.T) {
	ctx := loadFixture(t)
	config.Viper(ctx).Set(config.ChannelBuffer, 0)

	first := output.NewChannel(ctx, "first", nil, nil)
	second := output.NewChannel(ctx, "second", nil, nil)

	secondDone := make(chan struct{})
	go func() {
		for i := range 5 {
			second.WriteString("second line " + strconv.Itoa(i))
		}
		second.Close()
		close(secondDone)
	}()
	go func() {
		<-secondDone
		first.WriteString("first line")
		first.Close()
	}()

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	finished := make(chan struct{})
	go func() {
		output.NativeHandler(cmd, []output.Channel{first, second})
		close(finished)
	}()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("NativeHandler deadlocked waiting on the first repository")
	}

	out := buf.String()
	firstAt, secondAt := strings.Index(out, "------ first ------"), strings.Index(out, "------ second ------")
	if firstAt < 0 || secondAt < 0 || firstAt > secondAt {
		t.Fatalf("Expected first then second section, got:\n%s", out)
	}

	testhelper.AssertContains(t, out, []string{"first line", "second line 0", "second line 4"})
	testhelper.AssertContains(t, errBuf.String(), []string{"2 repositories | Elapsed:"})
}

// TestNativeHandler_PlainOutputAndSummary verifies escape sequences are stripped when
// stdout is not a terminal, and that errors and the summary name the failing repository.
func TestNativeHandler_PlainOutputAndSummary(t *testing.T) {
	ctx := loadFixture(t)
	testhelper.SetupDirs(t, ctx, []string{"repo1", "repo2"})
	config.Viper(ctx).Set(config.SortRepos, true)

	callFunc := func(_ context.Context, ch output.Channel) error {
		ch.WriteString("\x1b[32mmain\x1b[m ok")
		if ch.Name() == "repo2" {
			return errors.New("boom")
		}
		return nil
	}

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	if err := call.Do(cmd, []string{"repo1", "repo2"}, callFunc, output.NativeHandler); err == nil {
		t.Fatal("Expected Do to report the failure")
	}

	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("Expected escape sequences to be stripped, got %q", buf.String())
	}

	testhelper.AssertContains(t, buf.String(), []string{"main ok"})
	testhelper.AssertContains(t, errBuf.String(), []string{
		"ERROR: repo2: boom",
		"2 repositories (1 failed) | Elapsed:",
		"Failed: repo2",
	})
}

// TestNativeHandler_NoRepositories verifies an empty selection is reported.
func TestNativeHandler_NoRepositories(t *testing.T) {
	ctx := loadFixture(t)

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	output.NativeHandler(cmd, nil)

	testhelper.AssertContains(t, errBuf.String(), []string{"No repositories matched"})
}

// TestNativeLabels_VerboseLabelDetails verifies verbose output resolves filtered labels
// to their repositories, includes unwanted labels added as exclusions, and does not
// treat plain repository names as labels.
func TestNativeLabels_VerboseLabelDetails(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)
	viper.Set(config.SkipUnwanted, true)
	viper.Set(config.UnwantedLabels, []string{"deprecated"})
	viper.Set(config.SortRepos, true)

	catalog.Labels = map[string]mapset.Set[string]{
		"backend":    mapset.NewSet("api", "worker", "legacy"),
		"deprecated": mapset.NewSet("legacy"),
	}
	catalog.Catalog = map[string]scm.Repository{
		"api":    {Name: "api", Labels: []string{"backend"}},
		"worker": {Name: "worker", Labels: []string{"backend"}},
		"legacy": {Name: "legacy", Labels: []string{"backend", "deprecated"}},
	}

	var buf bytes.Buffer
	output.NativeLabels(fakeCmd(t, ctx, &buf), true, "~backend", "+worker")

	out := buf.String()
	testhelper.AssertContains(t, out, []string{
		"Included labels:\n  ~ backend ~ (2 / 3)\napi, legacy (unwanted), worker",
		"Excluded labels:\n  ~ deprecated ~ (1) (unwanted)\nlegacy",
	})
	testhelper.AssertNotContains(t, out, []string{"(empty label)", "Forced labels:"})
}

// TestNativeLabels_ListHidesUnwanted verifies that unwanted labels are hidden from the
// label list unless verbose, and are marked when shown.
func TestNativeLabels_ListHidesUnwanted(t *testing.T) {
	ctx := loadFixture(t)
	config.Viper(ctx).Set(config.UnwantedLabels, []string{"deprecated"})

	catalog.Labels = map[string]mapset.Set[string]{
		"backend":    mapset.NewSet("api", "legacy"),
		"deprecated": mapset.NewSet("legacy"),
	}

	var buf bytes.Buffer
	output.NativeLabels(fakeCmd(t, ctx, &buf), false)
	testhelper.AssertContains(t, buf.String(), []string{"~ backend ~ (1 / 2)"})
	testhelper.AssertNotContains(t, buf.String(), []string{"~ deprecated ~"})

	buf.Reset()
	output.NativeLabels(fakeCmd(t, ctx, &buf), true)
	testhelper.AssertContains(t, buf.String(), []string{"~ deprecated ~ (1) (unwanted)"})
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

// TestNativeCatalog_Archived verifies the native catalog renderer appends
// an "(archived)" indicator for archived repositories.
func TestNativeCatalog_Archived(t *testing.T) {
	ctx := loadFixture(t)

	catalog.Catalog = map[string]scm.Repository{
		"active-repo": {
			Name:          "active-repo",
			Description:   "Active repo",
			Project:       "test-project",
			DefaultBranch: "main",
			Public:        true,
		},
		"archived-repo": {
			Name:          "archived-repo",
			Description:   "Archived repo",
			Project:       "test-project",
			DefaultBranch: "main",
			Public:        false,
			Archived:      true,
		},
	}

	var buf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	output.NativeCatalog(cmd)

	out := buf.String()

	testhelper.AssertContains(t, out, []string{
		"## active-repo",
		"## archived-repo",
		"Visibility: public",
		"Visibility: private (archived)",
	})

	if bytes.Contains([]byte(out), []byte("Visibility: public (archived)")) {
		t.Error("Did not expect active-repo to be marked (archived)")
	}
}
