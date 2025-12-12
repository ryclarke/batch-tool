package output

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
)

func setupLabelsTest(t *testing.T) context.Context {
	t.Helper()
	ctx := loadFixture(t)

	// Set up mock catalog
	catalog.Labels = make(map[string]mapset.Set[string])
	catalog.Labels["core"] = mapset.NewSet("repo1", "repo2", "repo3")
	catalog.Labels["canary"] = mapset.NewSet("repo4", "repo5")
	catalog.Labels["csp"] = mapset.NewSet("repo6")
	catalog.Labels["empty-label"] = mapset.NewSet[string]()
	catalog.Labels["unwanted1"] = mapset.NewSet("repo7", "repo8")

	viper := config.Viper(ctx)
	viper.Set(config.UnwantedLabels, []string{"unwanted1"})
	viper.Set(config.SortRepos, true)
	viper.Set(config.SuperSetLabel, "all")

	return ctx
}

func TestNewLabelsListModel(t *testing.T) {
	ctx := setupLabelsTest(t)

	tests := []struct {
		name            string
		verbose         bool
		wantLabelCount  int
		wantUnwantedInc bool
	}{
		{
			name:            "non-verbose excludes unwanted",
			verbose:         false,
			wantLabelCount:  4, // core, canary, csp, empty-label (unwanted1 excluded)
			wantUnwantedInc: false,
		},
		{
			name:            "verbose includes unwanted",
			verbose:         true,
			wantLabelCount:  5, // core, canary, csp, empty-label, unwanted1
			wantUnwantedInc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLabelsListModel(ctx, tt.verbose)

			if len(m.labels) != tt.wantLabelCount {
				t.Errorf("Expected %d labels, got %d", tt.wantLabelCount, len(m.labels))
			}

			// Check if unwanted label is included based on verbose flag
			hasUnwanted := false
			for _, label := range m.labels {
				if label.name == "unwanted1" {
					hasUnwanted = true
					if !label.isUnwanted {
						t.Error("Expected unwanted1 to be marked as unwanted")
					}
				}
			}

			if hasUnwanted != tt.wantUnwantedInc {
				t.Errorf("Expected unwanted label inclusion: %v, got: %v", tt.wantUnwantedInc, hasUnwanted)
			}

			// Verify context is set
			if m.ctx != ctx {
				t.Error("Context not properly set")
			}

			// Verify verbose flag is set
			if m.verbose != tt.verbose {
				t.Errorf("Expected verbose=%v, got %v", tt.verbose, m.verbose)
			}
		})
	}
}

func TestLabelsListModelInit(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsListModel(ctx, false)

	cmd := m.Init()
	if cmd != nil {
		t.Error("Expected Init to return nil")
	}
}

func TestLabelsListModelUpdate(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsListModel(ctx, false)

	tests := []struct {
		name     string
		msg      tea.Msg
		wantQuit bool
	}{
		{
			name: "window size message",
			msg: tea.WindowSizeMsg{
				Width:  100,
				Height: 50,
			},
			wantQuit: false,
		},
		{
			name:     "quit with q",
			msg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			wantQuit: true,
		},
		{
			name:     "quit with esc",
			msg:      tea.KeyMsg{Type: tea.KeyEsc},
			wantQuit: true,
		},
		{
			name:     "quit with ctrl+c",
			msg:      tea.KeyMsg{Type: tea.KeyCtrlC},
			wantQuit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedModel, cmd := m.Update(tt.msg)

			if tt.wantQuit {
				if cmd == nil || cmd() != tea.Quit() {
					t.Error("Expected quit command")
				}
			}

			// Verify model is updated
			if updatedModel == nil {
				t.Error("Expected non-nil model")
			}
		})
	}
}

func TestLabelsListModelBuildContent(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsListModel(ctx, true) // verbose to include unwanted
	m.width = 100

	content := m.buildContent()

	// Verify content contains label names
	if !strings.Contains(content, "core") {
		t.Error("Expected content to contain 'core' label")
	}

	if !strings.Contains(content, "canary") {
		t.Error("Expected content to contain 'canary' label")
	}

	// Verify empty label shows correct text
	if !strings.Contains(content, emptyLabelText) {
		t.Error("Expected content to show empty label text")
	}

	// Verify unwanted label is rendered
	if !strings.Contains(content, "unwanted1") {
		t.Error("Expected content to contain unwanted label")
	}

	// Verify separators are present
	if !strings.Contains(content, separatorLine) {
		t.Error("Expected content to contain separators")
	}
}

func TestLabelsListModelView(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsListModel(ctx, false)

	// Test before ready
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Error("Expected loading message when not ready")
	}

	// Initialize with window size to make ready
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updatedModel.(labelsListModel)

	// Test after ready
	view = m.View()
	if !strings.Contains(view, "Available Labels") {
		t.Error("Expected title 'Available Labels' in view")
	}

	if !strings.Contains(view, footerDone) {
		t.Error("Expected footer in view")
	}
}

func TestNewLabelsFilterModel(t *testing.T) {
	ctx := setupLabelsTest(t)

	tests := []struct {
		name    string
		filters []string
		verbose bool
	}{
		{
			name:    "single filter non-verbose",
			filters: []string{"core"},
			verbose: false,
		},
		{
			name:    "multiple filters verbose",
			filters: []string{"core", "canary"},
			verbose: true,
		},
		{
			name:    "filter with exclusion",
			filters: []string{"core", "~canary"},
			verbose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLabelsFilterModel(ctx, tt.verbose, tt.filters)

			if m.ctx != ctx {
				t.Error("Context not properly set")
			}

			if m.verbose != tt.verbose {
				t.Errorf("Expected verbose=%v, got %v", tt.verbose, m.verbose)
			}

			if len(m.filters) != len(tt.filters) {
				t.Errorf("Expected %d filters, got %d", len(tt.filters), len(m.filters))
			}

			// If verbose, check that labels are initialized
			if tt.verbose {
				totalLabels := len(m.labels.forced) + len(m.labels.included) + len(m.labels.excluded)
				if totalLabels == 0 {
					t.Error("Expected labels to be initialized in verbose mode")
				}
			}
		})
	}
}

func TestLabelsFilterModelInit(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsFilterModel(ctx, false, []string{"core"})

	cmd := m.Init()
	if cmd != nil {
		t.Error("Expected Init to return nil")
	}
}

func TestLabelsFilterModelUpdate(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsFilterModel(ctx, false, []string{"core"})

	tests := []struct {
		name     string
		msg      tea.Msg
		wantQuit bool
	}{
		{
			name: "window size message",
			msg: tea.WindowSizeMsg{
				Width:  100,
				Height: 50,
			},
			wantQuit: false,
		},
		{
			name:     "quit with q",
			msg:      tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}},
			wantQuit: true,
		},
		{
			name:     "quit with esc",
			msg:      tea.KeyMsg{Type: tea.KeyEsc},
			wantQuit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedModel, cmd := m.Update(tt.msg)

			if tt.wantQuit {
				if cmd == nil || cmd() != tea.Quit() {
					t.Error("Expected quit command")
				}
			}

			if updatedModel == nil {
				t.Error("Expected non-nil model")
			}
		})
	}
}

func TestLabelsFilterModelBuildSetString(t *testing.T) {
	ctx := setupLabelsTest(t)

	tests := []struct {
		name     string
		filters  []string
		wantText []string
	}{
		{
			name:     "simple inclusion",
			filters:  []string{"core"},
			wantText: []string{"core"},
		},
		{
			name:     "with exclusion",
			filters:  []string{"core", "~canary"},
			wantText: []string{"core", "canary", "∖"},
		},
		{
			name:     "with forced",
			filters:  []string{"+core", "canary"},
			wantText: []string{"core", "canary", "∪"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLabelsFilterModel(ctx, false, tt.filters)
			m.width = 100

			setString := m.buildSetString()

			for _, want := range tt.wantText {
				if !strings.Contains(setString, want) {
					t.Errorf("Expected set string to contain %q", want)
				}
			}
		})
	}
}

func TestLabelsFilterModelBuildContent(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsFilterModel(ctx, true, []string{"core"})
	m.width = 100

	content := m.buildContent(ctx)

	// Verify it shows repository matches
	if !strings.Contains(content, "This matches") {
		t.Error("Expected content to contain match summary")
	}

	if !strings.Contains(content, "repositor") {
		t.Error("Expected content to mention repositories")
	}
}

func TestLabelsFilterModelView(t *testing.T) {
	ctx := setupLabelsTest(t)
	m := newLabelsFilterModel(ctx, false, []string{"core"})

	// Test before ready
	view := m.View()
	if !strings.Contains(view, "Processing") {
		t.Error("Expected processing message when not ready")
	}

	// Initialize with window size to make ready
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updatedModel.(labelsFilterModel)

	// Test after ready
	view = m.View()
	if !strings.Contains(view, "Selected Set") {
		t.Error("Expected title 'Selected Set' in view")
	}

	if !strings.Contains(view, footerDone) {
		t.Error("Expected footer in view")
	}
}

func TestBuildLabelWithRepos(t *testing.T) {
	ctx := setupLabelsTest(t)

	tests := []struct {
		name       string
		labelNames []string
		wantCount  int
		wantEmpty  bool
	}{
		{
			name:       "empty input",
			labelNames: []string{},
			wantCount:  0,
			wantEmpty:  true,
		},
		{
			name:       "single label with repos",
			labelNames: []string{"core"},
			wantCount:  1,
			wantEmpty:  false,
		},
		{
			name:       "empty label",
			labelNames: []string{"empty-label"},
			wantCount:  1,
			wantEmpty:  true,
		},
		{
			name:       "multiple labels",
			labelNames: []string{"core", "canary", "csp"},
			wantCount:  3,
			wantEmpty:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLabelWithRepos(ctx, tt.labelNames)

			if len(result) != tt.wantCount {
				t.Errorf("Expected %d labels, got %d", tt.wantCount, len(result))
			}

			if tt.wantEmpty && len(result) > 0 {
				if !result[0].empty {
					t.Error("Expected label to be marked as empty")
				}
			}

			// Verify repos are sorted when SortRepos is enabled
			for _, label := range result {
				if len(label.repos) > 1 {
					for i := 1; i < len(label.repos); i++ {
						if label.repos[i-1] > label.repos[i] {
							t.Error("Expected repos to be sorted")
							break
						}
					}
				}
			}
		})
	}
}

func TestLabelWithRepos(t *testing.T) {
	tests := []struct {
		name       string
		label      labelWithRepos
		wantName   string
		wantEmpty  bool
		wantUnwant bool
	}{
		{
			name: "normal label with repos",
			label: labelWithRepos{
				name:       "core",
				repos:      []string{"repo1", "repo2"},
				empty:      false,
				isUnwanted: false,
			},
			wantName:   "core",
			wantEmpty:  false,
			wantUnwant: false,
		},
		{
			name: "empty unwanted label",
			label: labelWithRepos{
				name:       "unwanted1",
				repos:      []string{},
				empty:      true,
				isUnwanted: true,
			},
			wantName:   "unwanted1",
			wantEmpty:  true,
			wantUnwant: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.label.name != tt.wantName {
				t.Errorf("Expected name %q, got %q", tt.wantName, tt.label.name)
			}

			if tt.label.empty != tt.wantEmpty {
				t.Errorf("Expected empty=%v, got %v", tt.wantEmpty, tt.label.empty)
			}

			if tt.label.isUnwanted != tt.wantUnwant {
				t.Errorf("Expected isUnwanted=%v, got %v", tt.wantUnwant, tt.label.isUnwanted)
			}
		})
	}
}
