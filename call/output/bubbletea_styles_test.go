package output

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
)

func TestNewOutputStyles(t *testing.T) {
	width := 100
	styles := newOutputStyles(width)

	// Verify key styles are initialized
	if styles.output.GetWidth() != width-4 {
		t.Errorf("Expected output style width %d, got %d", width-4, styles.output.GetWidth())
	}
}

func TestNewLabelStyles(t *testing.T) {
	width := 100
	styles := newLabelStyles(width)

	// Verify wrap function is initialized
	if styles.wrap == nil {
		t.Error("Expected wrap function to be initialized")
	}

	// Verify key styles exist (no width check as they don't have width set)
}

func TestHandleViewportKeyPress(t *testing.T) {
	vp := viewport.New(80, 24)
	vp.SetContent(strings.Repeat("line\n", 100))

	tests := []struct {
		name        string
		key         string
		wantHandled bool
	}{
		// Single line scrolling
		{"down arrow", "down", true},
		{"up arrow", "up", true},
		{"j key", "j", true},
		{"k key", "k", true},

		// Half page scrolling
		{"shift+down", "shift+down", true},
		{"shift+up", "shift+up", true},
		{"J key", "J", true},
		{"K key", "K", true},
		{"shift+j", "shift+j", true},
		{"shift+k", "shift+k", true},
		{"ctrl+d", "ctrl+d", true},
		{"ctrl+u", "ctrl+u", true},

		// Full page scrolling
		{"pgdown", "pgdown", true},
		{"pgup", "pgup", true},
		{"ctrl+f", "ctrl+f", true},
		{"ctrl+b", "ctrl+b", true},

		// Jump to top/bottom
		{"home", "home", true},
		{"end", "end", true},
		{"g key", "g", true},
		{"G key", "G", true},
		{"shift+g", "shift+g", true},

		// Unhandled keys
		{"q key", "q", false},
		{"esc key", "esc", false},
		{"random key", "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handled := handleKeyPress(&vp, tt.key)
			if handled != tt.wantHandled {
				t.Errorf("handleViewportKeyPress(%q) = %v, want %v", tt.key, handled, tt.wantHandled)
			}
		})
	}
}

func TestGetUnwantedRepos(t *testing.T) {
	ctx := config.LoadFixture(t, "../../config")
	viper := config.Viper(ctx)

	// Set up unwanted labels in config
	unwantedLabels := []string{"unwanted1", "unwanted2"}
	viper.Set(config.UnwantedLabels, unwantedLabels)

	// Create mock catalog with unwanted labels
	catalog.Labels = make(map[string]mapset.Set[string])
	catalog.Labels["unwanted1"] = mapset.NewSet("repo1", "repo2")
	catalog.Labels["unwanted2"] = mapset.NewSet("repo3", "repo4")
	catalog.Labels["wanted"] = mapset.NewSet("repo5", "repo6")

	unwanted := getUnwantedRepos(ctx)

	// Verify unwanted repos are included
	if !unwanted.Contains("repo1") {
		t.Error("Expected repo1 to be in unwanted set")
	}
	if !unwanted.Contains("repo3") {
		t.Error("Expected repo3 to be in unwanted set")
	}

	// Verify wanted repos are not included
	if unwanted.Contains("repo5") {
		t.Error("Expected repo5 to NOT be in unwanted set")
	}
}

func TestIsLabelUnwanted(t *testing.T) {
	ctx := config.LoadFixture(t, "../../config")
	viper := config.Viper(ctx)

	unwantedLabels := []string{"unwanted1", "unwanted2"}
	viper.Set(config.UnwantedLabels, unwantedLabels)

	tests := []struct {
		name       string
		labelName  string
		wantResult bool
	}{
		{
			name:       "unwanted label",
			labelName:  "unwanted1",
			wantResult: true,
		},
		{
			name:       "another unwanted label",
			labelName:  "unwanted2",
			wantResult: true,
		},
		{
			name:       "wanted label",
			labelName:  "wanted",
			wantResult: false,
		},
		{
			name:       "non-existent label",
			labelName:  "nonexistent",
			wantResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLabelUnwanted(ctx, tt.labelName)
			if result != tt.wantResult {
				t.Errorf("isLabelUnwanted(%q) = %v, want %v", tt.labelName, result, tt.wantResult)
			}
		})
	}
}
