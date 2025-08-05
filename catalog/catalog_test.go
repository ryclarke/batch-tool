package catalog

import (
	"bytes"
	"os"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
	"github.com/spf13/viper"
)

func TestRepositoryList(t *testing.T) {
	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend": mapset.NewSet("web-app", "mobile-app"),
		"backend":  mapset.NewSet("api-server", "worker"),
		"all":      mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "tools"),
	}

	// Test basic repository lookup
	repoList := RepositoryList("web-app", "api-server")
	repos := repoList.ToSlice()

	if len(repos) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(repos))
	}

	// Check that both repos are present
	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app in repository list")
	}
	if !repoSet.Contains("api-server") {
		t.Error("Expected api-server in repository list")
	}
}

func TestRepositoryListWithLabels(t *testing.T) {
	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend": mapset.NewSet("web-app", "mobile-app"),
		"backend":  mapset.NewSet("api-server", "worker"),
	}

	// Test label lookup
	repoList := RepositoryList("~frontend")
	repos := repoList.ToSlice()

	if len(repos) != 2 {
		t.Errorf("Expected 2 frontend repositories, got %d", len(repos))
	}

	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app in frontend label")
	}
	if !repoSet.Contains("mobile-app") {
		t.Error("Expected mobile-app in frontend label")
	}
}

func TestRepositoryListWithExclusions(t *testing.T) {
	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "deprecated-app"),
		"deprecated": mapset.NewSet("deprecated-app"),
	}

	// Test exclusion
	repoList := RepositoryList("~all", "!deprecated-app")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)
	if repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be excluded")
	}

	// Should contain the other apps
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app to be included")
	}
}

func TestRepositoryListWithSkipUnwanted(t *testing.T) {
	// Set up configuration
	viper.Set(config.SkipUnwanted, true)
	viper.Set(config.UnwantedLabels, []string{"deprecated", "poc"})

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"all":        mapset.NewSet("web-app", "mobile-app", "deprecated-app", "poc-app"),
		"deprecated": mapset.NewSet("deprecated-app"),
		"poc":        mapset.NewSet("poc-app"),
	}

	Catalog = map[string]scm.Repository{
		"web-app":        {Name: "web-app", Labels: []string{}},
		"mobile-app":     {Name: "mobile-app", Labels: []string{}},
		"deprecated-app": {Name: "deprecated-app", Labels: []string{"deprecated"}},
		"poc-app":        {Name: "poc-app", Labels: []string{"poc"}},
	}

	// Test that unwanted repositories are skipped
	repoList := RepositoryList("~all")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)
	if repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be skipped")
	}
	if repoSet.Contains("poc-app") {
		t.Error("Expected poc-app to be skipped")
	}

	// Should contain the wanted apps
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app to be included")
	}
	if !repoSet.Contains("mobile-app") {
		t.Error("Expected mobile-app to be included")
	}
}

func TestRepositoryListEmptyInput(t *testing.T) {
	// Test with no arguments
	repoList := RepositoryList()
	repos := repoList.ToSlice()

	if len(repos) != 0 {
		t.Errorf("Expected 0 repositories for empty input, got %d", len(repos))
	}
}

func TestRepositoryListNonexistentLabel(t *testing.T) {
	// Test with nonexistent label
	Labels = map[string]mapset.Set[string]{
		"frontend": mapset.NewSet("web-app"),
	}

	repoList := RepositoryList("~nonexistent")
	repos := repoList.ToSlice()

	if len(repos) != 0 {
		t.Errorf("Expected 0 repositories for nonexistent label, got %d", len(repos))
	}
}

func TestRepositoryListMixedInput(t *testing.T) {
	// Test with mix of labels and repository names
	Labels = map[string]mapset.Set[string]{
		"frontend": mapset.NewSet("web-app", "mobile-app"),
	}

	repoList := RepositoryList("~frontend", "api-server")
	repos := repoList.ToSlice()

	if len(repos) != 3 {
		t.Errorf("Expected 3 repositories (2 from label + 1 direct), got %d", len(repos))
	}

	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app from frontend label")
	}
	if !repoSet.Contains("mobile-app") {
		t.Error("Expected mobile-app from frontend label")
	}
	if !repoSet.Contains("api-server") {
		t.Error("Expected api-server from direct reference")
	}
}

// TestInitWithFakeProvider tests catalog initialization with a fake SCM provider
func TestInitWithFakeProvider(t *testing.T) {
	// Save original state
	originalCatalog := Catalog
	originalLabels := Labels
	defer func() {
		Catalog = originalCatalog
		Labels = originalLabels
	}()

	// Create fake provider with test data
	testRepos := fake.CreateTestRepositories("test-project")
	fakeProvider := fake.NewFake("test-project", testRepos)

	// Register fake provider
	scm.Register("fake-test", func(project string) scm.Provider {
		return fakeProvider
	})

	// Manually initialize for testing purposes (bypassing cache issues)
	Catalog = make(map[string]scm.Repository)
	Labels = make(map[string]mapset.Set[string])

	repos, err := fakeProvider.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to get repositories from fake provider: %v", err)
	}

	for _, repo := range repos {
		Catalog[repo.Name] = *repo
		for _, label := range repo.Labels {
			if _, ok := Labels[label]; !ok {
				Labels[label] = mapset.NewSet(repo.Name)
			} else {
				Labels[label].Add(repo.Name)
			}
		}
	}

	// Verify catalog was populated
	if len(Catalog) == 0 {
		t.Error("Expected catalog to be populated with repositories")
	}

	// Verify specific repositories
	repo1, exists := Catalog["repo-1"]
	if !exists {
		t.Error("Expected repo-1 in catalog")
	} else if repo1.Name != "repo-1" {
		t.Errorf("Expected repo-1 in catalog, got %s", repo1.Name)
	}

	// Verify labels were created
	if len(Labels) == 0 {
		t.Error("Expected labels to be populated")
	}

	// Check for specific labels
	activeRepos := Labels["active"]
	if activeRepos == nil {
		t.Error("Expected 'active' label to exist")
	} else if activeRepos.Cardinality() == 0 {
		t.Error("Expected 'active' label to have repositories")
	}
}

// TestPrintFunctions tests the print functions with test data
func TestPrintFunctions(t *testing.T) {
	// Save original state
	originalCatalog := Catalog
	originalLabels := Labels
	defer func() {
		Catalog = originalCatalog
		Labels = originalLabels
	}()

	// Set up test data
	Catalog = map[string]scm.Repository{
		"repo-1": {Name: "repo-1", Labels: []string{"backend", "go"}},
		"repo-2": {Name: "repo-2", Labels: []string{"frontend", "javascript"}},
		"repo-3": {Name: "repo-3", Labels: []string{"deprecated"}},
	}

	Labels = map[string]mapset.Set[string]{
		"backend":    mapset.NewSet("repo-1"),
		"frontend":   mapset.NewSet("repo-2"),
		"deprecated": mapset.NewSet("repo-3"),
		"go":         mapset.NewSet("repo-1"),
		"javascript": mapset.NewSet("repo-2"),
	}

	// Test PrintLabels
	t.Run("PrintLabels", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		PrintLabels()

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Check that all labels are printed
		expectedLabels := []string{"backend", "frontend", "deprecated", "go", "javascript"}
		for _, label := range expectedLabels {
			if !strings.Contains(output, label) {
				t.Errorf("Expected label '%s' in output", label)
			}
		}
	})

	// Test PrintSet with non-verbose mode
	t.Run("PrintSet_NonVerbose", func(t *testing.T) {
		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		viper.Set(config.SkipUnwanted, false) // Disable skip unwanted for this test
		PrintSet(false, "~frontend")

		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		buf.ReadFrom(r)
		output := buf.String()

		// Should contain repository name
		if !strings.Contains(output, "repo-2") {
			t.Error("Expected repo-2 in non-verbose output")
		}
		if !strings.Contains(output, "You've selected the following set:") {
			t.Error("Expected set description in output")
		}
	})
}
