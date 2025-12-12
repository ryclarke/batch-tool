package output

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

func setupCatalogTest(t *testing.T) context.Context {
	t.Helper()
	ctx := loadFixture(t)

	// Set up mock catalog
	catalog.Catalog = make(map[string]scm.Repository)
	catalog.Catalog["repo1"] = scm.Repository{
		Name:          "repo1",
		Description:   "First test repository",
		Project:       "test-project",
		DefaultBranch: "main",
		Labels:        []string{"core", "backend"},
		Public:        true,
	}
	catalog.Catalog["repo2"] = scm.Repository{
		Name:          "repo2",
		Description:   "Second test repository",
		Project:       "test-project",
		DefaultBranch: "master",
		Labels:        []string{"frontend", "experimental"},
		Public:        false,
	}
	catalog.Catalog["repo3"] = scm.Repository{
		Name:          "repo3",
		Description:   "",
		Project:       "other-project",
		DefaultBranch: "develop",
		Labels:        []string{},
		Public:        true,
	}

	viper := config.Viper(ctx)
	viper.Set(config.SortRepos, true)

	return ctx
}

func TestNewCatalogModel(t *testing.T) {
	ctx := setupCatalogTest(t)

	m := newCatalogModel(ctx)

	if m.ctx != ctx {
		t.Error("Context not properly set")
	}

	if len(m.repos) != 3 {
		t.Errorf("Expected 3 repos, got %d", len(m.repos))
	}

	// Verify repos are sorted
	if len(m.repos) > 0 && m.repos[0].name != "repo1" {
		t.Errorf("Expected first repo to be 'repo1', got '%s'", m.repos[0].name)
	}

	// Verify metadata is populated
	for _, repo := range m.repos {
		if repo.name == "repo1" {
			if repo.description != "First test repository" {
				t.Errorf("Expected description 'First test repository', got '%s'", repo.description)
			}
			if repo.project != "test-project" {
				t.Errorf("Expected project 'test-project', got '%s'", repo.project)
			}
			if repo.defaultBranch != "main" {
				t.Errorf("Expected branch 'main', got '%s'", repo.defaultBranch)
			}
			if !repo.isPublic {
				t.Error("Expected repo1 to be public")
			}
			if len(repo.labels) != 2 {
				t.Errorf("Expected 2 labels, got %d", len(repo.labels))
			}
		}
	}
}

func TestCatalogModelInit(t *testing.T) {
	ctx := setupCatalogTest(t)
	m := newCatalogModel(ctx)

	cmd := m.Init()
	if cmd != nil {
		t.Error("Expected Init to return nil")
	}
}

func TestCatalogModelUpdate(t *testing.T) {
	ctx := setupCatalogTest(t)
	m := newCatalogModel(ctx)

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

			if updatedModel == nil {
				t.Error("Expected non-nil model")
			}
		})
	}
}

func TestCatalogModelBuildContent(t *testing.T) {
	ctx := setupCatalogTest(t)
	m := newCatalogModel(ctx)
	m.width = 100

	content := m.buildContent()

	// Verify content contains repository names
	if !strings.Contains(content, "repo1") {
		t.Error("Expected content to contain 'repo1'")
	}

	if !strings.Contains(content, "repo2") {
		t.Error("Expected content to contain 'repo2'")
	}

	if !strings.Contains(content, "repo3") {
		t.Error("Expected content to contain 'repo3'")
	}

	// Verify descriptions are present
	if !strings.Contains(content, "First test repository") {
		t.Error("Expected content to contain repo1 description")
	}

	// Verify metadata
	if !strings.Contains(content, "Project:") {
		t.Error("Expected content to contain 'Project:' label")
	}

	if !strings.Contains(content, "Default Branch:") {
		t.Error("Expected content to contain 'Default Branch:' label")
	}

	if !strings.Contains(content, "Visibility:") {
		t.Error("Expected content to contain 'Visibility:' label")
	}

	// Verify visibility values
	if !strings.Contains(content, "public") {
		t.Error("Expected content to contain 'public' visibility")
	}

	if !strings.Contains(content, "private") {
		t.Error("Expected content to contain 'private' visibility")
	}

	// Verify labels are present (shown inline with parentheses)
	if !strings.Contains(content, "core") {
		t.Error("Expected content to contain 'core' label")
	}

	// Verify separators
	if !strings.Contains(content, separatorLine) {
		t.Error("Expected content to contain separators")
	}
}

func TestCatalogModelView(t *testing.T) {
	ctx := setupCatalogTest(t)
	m := newCatalogModel(ctx)

	// Test before ready
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Error("Expected loading message when not ready")
	}

	// Initialize with window size to make ready
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updatedModel.(catalogModel)

	// Test after ready
	view = m.View()
	if !strings.Contains(view, "Repository Catalog") {
		t.Error("Expected title 'Repository Catalog' in view")
	}

	// Verify repository count in title
	if !strings.Contains(view, "(3 repositories)") {
		t.Error("Expected repository count in title")
	}

	if !strings.Contains(view, footerDone) {
		t.Error("Expected footer in view")
	}
}

func TestNewCatalogStyles(t *testing.T) {
	width := 100
	styles := newCatalogStyles(width)

	// Verify key width-based styles are initialized
	if styles.description.GetWidth() != width-4 {
		t.Errorf("Expected description width %d, got %d", width-4, styles.description.GetWidth())
	}
}

func TestRepoWithMetadata(t *testing.T) {
	repo := repoWithMetadata{
		name:          "test-repo",
		description:   "Test description",
		project:       "test-project",
		defaultBranch: "main",
		labels:        []string{"label1", "label2"},
		isPublic:      true,
	}

	if repo.name != "test-repo" {
		t.Errorf("Expected name 'test-repo', got '%s'", repo.name)
	}

	if repo.description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", repo.description)
	}

	if !repo.isPublic {
		t.Error("Expected isPublic to be true")
	}

	if len(repo.labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(repo.labels))
	}
}
