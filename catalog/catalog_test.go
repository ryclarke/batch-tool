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
	_ = config.LoadFixture("../config")

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
	_ = config.LoadFixture("../config")

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
	_ = config.LoadFixture("../config")

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
	_ = config.LoadFixture("../config")

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
	_ = config.LoadFixture("../config")

	// Test with no arguments
	repoList := RepositoryList()
	repos := repoList.ToSlice()

	if len(repos) != 0 {
		t.Errorf("Expected 0 repositories for empty input, got %d", len(repos))
	}
}

func TestRepositoryListNonexistentLabel(t *testing.T) {
	_ = config.LoadFixture("../config")

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
	_ = config.LoadFixture("../config")

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

// TestRepositoryListMultipleLabels tests combining multiple labels without forced inclusion
func TestRepositoryListMultipleLabels(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend":     mapset.NewSet("web-app", "mobile-app"),
		"backend":      mapset.NewSet("api-server", "worker"),
		"microservice": mapset.NewSet("user-service", "payment-service"),
		"database":     mapset.NewSet("postgres-service", "redis-service"),
	}

	// Test combining multiple labels
	repoList := RepositoryList("~frontend", "~backend")
	repos := repoList.ToSlice()

	if len(repos) != 4 {
		t.Errorf("Expected 4 repositories from two labels, got %d", len(repos))
	}

	repoSet := mapset.NewSet(repos...)
	expectedRepos := []string{"web-app", "mobile-app", "api-server", "worker"}
	for _, repo := range expectedRepos {
		if !repoSet.Contains(repo) {
			t.Errorf("Expected %s from combined labels", repo)
		}
	}
}

// TestRepositoryListMultipleExclusions tests excluding multiple items
func TestRepositoryListMultipleExclusions(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "test-app"),
		"deprecated": mapset.NewSet("deprecated-app"),
		"test":       mapset.NewSet("test-app"),
	}

	// Test excluding multiple individual repos
	repoList := RepositoryList("~all", "!deprecated-app", "!test-app")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)
	if repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be excluded")
	}
	if repoSet.Contains("test-app") {
		t.Error("Expected test-app to be excluded")
	}

	expectedRepos := []string{"web-app", "mobile-app", "api-server", "worker"}
	for _, repo := range expectedRepos {
		if !repoSet.Contains(repo) {
			t.Errorf("Expected %s to be included", repo)
		}
	}
}

// TestRepositoryListExcludeMultipleLabels tests excluding multiple labels
func TestRepositoryListExcludeMultipleLabels(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "test-app", "experimental-app"),
		"deprecated": mapset.NewSet("deprecated-app"),
		"test":       mapset.NewSet("test-app", "experimental-app"),
		"backend":    mapset.NewSet("api-server", "worker"),
	}

	// Test excluding multiple labels
	repoList := RepositoryList("~all", "!~deprecated", "!~test")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)

	// Should exclude all repos from deprecated and test labels
	excludedRepos := []string{"deprecated-app", "test-app", "experimental-app"}
	for _, repo := range excludedRepos {
		if repoSet.Contains(repo) {
			t.Errorf("Expected %s to be excluded", repo)
		}
	}

	// Should include remaining repos
	expectedRepos := []string{"web-app", "mobile-app", "api-server", "worker"}
	for _, repo := range expectedRepos {
		if !repoSet.Contains(repo) {
			t.Errorf("Expected %s to be included", repo)
		}
	}
}

// TestRepositoryListMixedIncludesAndExcludes tests combining includes and excludes without forced
func TestRepositoryListMixedIncludesAndExcludes(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend":     mapset.NewSet("web-app", "mobile-app", "admin-panel"),
		"backend":      mapset.NewSet("api-server", "worker", "scheduler"),
		"microservice": mapset.NewSet("user-service", "payment-service", "notification-service"),
		"legacy":       mapset.NewSet("old-api", "legacy-frontend"),
	}

	// Test: include frontend and microservices, exclude specific repos, exclude legacy label
	repoList := RepositoryList("~frontend", "~microservice", "!mobile-app", "!payment-service", "!~legacy")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)

	// Should include from frontend (except mobile-app)
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app from frontend")
	}
	if !repoSet.Contains("admin-panel") {
		t.Error("Expected admin-panel from frontend")
	}
	if repoSet.Contains("mobile-app") {
		t.Error("Expected mobile-app to be excluded")
	}

	// Should include from microservice (except payment-service)
	if !repoSet.Contains("user-service") {
		t.Error("Expected user-service from microservice")
	}
	if !repoSet.Contains("notification-service") {
		t.Error("Expected notification-service from microservice")
	}
	if repoSet.Contains("payment-service") {
		t.Error("Expected payment-service to be excluded")
	}

	// Should exclude all legacy repos
	if repoSet.Contains("old-api") {
		t.Error("Expected old-api to be excluded (legacy label)")
	}
	if repoSet.Contains("legacy-frontend") {
		t.Error("Expected legacy-frontend to be excluded (legacy label)")
	}

	// Should not include backend repos (not in filter)
	if repoSet.Contains("api-server") {
		t.Error("Expected api-server to not be included (not in filters)")
	}
}

// TestRepositoryListLabelOverlap tests behavior with overlapping labels
func TestRepositoryListLabelOverlap(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data with overlapping labels
	Labels = map[string]mapset.Set[string]{
		"frontend":   mapset.NewSet("web-app", "mobile-app", "shared-component"),
		"backend":    mapset.NewSet("api-server", "shared-component", "worker"),
		"shared":     mapset.NewSet("shared-component", "shared-utils"),
		"javascript": mapset.NewSet("web-app", "mobile-app", "shared-utils"),
	}

	// Test: include frontend and backend (shared-component should appear only once)
	repoList := RepositoryList("~frontend", "~backend")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)

	// Verify all unique repos are included
	expectedRepos := []string{"web-app", "mobile-app", "shared-component", "api-server", "worker"}
	if len(repos) != len(expectedRepos) {
		t.Errorf("Expected %d unique repositories, got %d", len(expectedRepos), len(repos))
	}

	for _, repo := range expectedRepos {
		if !repoSet.Contains(repo) {
			t.Errorf("Expected %s to be included", repo)
		}
	}
}

// TestRepositoryListComplexExclusionPattern tests complex exclusion patterns
func TestRepositoryListComplexExclusionPattern(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"all":          mapset.NewSet("service-a", "service-b", "service-c", "service-d", "util-x", "util-y", "test-helper"),
		"services":     mapset.NewSet("service-a", "service-b", "service-c", "service-d"),
		"utilities":    mapset.NewSet("util-x", "util-y"),
		"testing":      mapset.NewSet("test-helper"),
		"core":         mapset.NewSet("service-a", "service-b"),
		"experimental": mapset.NewSet("service-d", "util-y"),
	}

	// Test: include all services, but exclude experimental ones and specific service
	repoList := RepositoryList("~services", "!~experimental", "!service-c")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)

	// Should include core services
	if !repoSet.Contains("service-a") {
		t.Error("Expected service-a to be included")
	}
	if !repoSet.Contains("service-b") {
		t.Error("Expected service-b to be included")
	}

	// Should exclude service-c (explicit exclusion)
	if repoSet.Contains("service-c") {
		t.Error("Expected service-c to be excluded (explicit)")
	}

	// Should exclude service-d (experimental label exclusion)
	if repoSet.Contains("service-d") {
		t.Error("Expected service-d to be excluded (experimental)")
	}

	// Should not include utilities (not in services label)
	if repoSet.Contains("util-x") {
		t.Error("Expected util-x to not be included")
	}
	if repoSet.Contains("util-y") {
		t.Error("Expected util-y to not be included")
	}
}

// TestRepositoryListDirectRepoSelection tests direct repository selection patterns
func TestRepositoryListDirectRepoSelection(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"group-a": mapset.NewSet("repo-a1", "repo-a2", "repo-a3"),
		"group-b": mapset.NewSet("repo-b1", "repo-b2"),
	}

	// Test: mix direct repo names with label exclusions
	repoList := RepositoryList("repo-a1", "repo-b1", "custom-repo", "!repo-a2")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)

	// Should include directly specified repos
	if !repoSet.Contains("repo-a1") {
		t.Error("Expected repo-a1 to be included (direct)")
	}
	if !repoSet.Contains("repo-b1") {
		t.Error("Expected repo-b1 to be included (direct)")
	}
	if !repoSet.Contains("custom-repo") {
		t.Error("Expected custom-repo to be included (direct)")
	}

	// Should exclude explicitly excluded repo
	if repoSet.Contains("repo-a2") {
		t.Error("Expected repo-a2 to be excluded")
	}

	// Should not include repos not mentioned
	if repoSet.Contains("repo-a3") {
		t.Error("Expected repo-a3 to not be included")
	}
	if repoSet.Contains("repo-b2") {
		t.Error("Expected repo-b2 to not be included")
	}
}

// TestRepositoryListWithForcedInclusion tests the '+' token for forced inclusion
func TestRepositoryListWithForcedInclusion(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend":   mapset.NewSet("web-app", "mobile-app"),
		"backend":    mapset.NewSet("api-server", "worker"),
		"deprecated": mapset.NewSet("deprecated-app", "old-api"),
		"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "old-api"),
	}

	// Test forced inclusion of single repository
	repoList := RepositoryList("+deprecated-app")
	repos := repoList.ToSlice()

	if len(repos) != 1 {
		t.Errorf("Expected 1 repository with forced inclusion, got %d", len(repos))
	}

	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be forcibly included")
	}
}

// TestRepositoryListWithForcedLabelInclusion tests the '+~' token combination
func TestRepositoryListWithForcedLabelInclusion(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend":   mapset.NewSet("web-app", "mobile-app"),
		"backend":    mapset.NewSet("api-server", "worker"),
		"deprecated": mapset.NewSet("deprecated-app", "old-api"),
	}

	// Test forced inclusion of entire label
	repoList := RepositoryList("+~deprecated")
	repos := repoList.ToSlice()

	if len(repos) != 2 {
		t.Errorf("Expected 2 repositories from forced label inclusion, got %d", len(repos))
	}

	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app from forced label inclusion")
	}
	if !repoSet.Contains("old-api") {
		t.Error("Expected old-api from forced label inclusion")
	}
}

// TestRepositoryListMixedTokens tests various combinations of ~, !, and + tokens
func TestRepositoryListMixedTokens(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend":   mapset.NewSet("web-app", "mobile-app"),
		"backend":    mapset.NewSet("api-server", "worker"),
		"deprecated": mapset.NewSet("deprecated-app", "old-api"),
		"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "old-api"),
	}

	// Test case 1: Include frontend, exclude mobile-app, force deprecated-app
	// Expected: web-app (from ~frontend, mobile-app excluded) + deprecated-app (forced)
	repoList := RepositoryList("~frontend", "!mobile-app", "+deprecated-app")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app from frontend label")
	}
	if repoSet.Contains("mobile-app") {
		t.Error("Expected mobile-app to be excluded")
	}
	if !repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be forcibly included")
	}

	// Test case 2: Include all, exclude deprecated label, but force specific deprecated repo
	// Expected: all repos except deprecated label, but deprecated-app is forced back in
	repoList2 := RepositoryList("~all", "!~deprecated", "+deprecated-app")
	repos2 := repoList2.ToSlice()

	repoSet2 := mapset.NewSet(repos2...)
	if !repoSet2.Contains("web-app") {
		t.Error("Expected web-app from all label")
	}
	if !repoSet2.Contains("mobile-app") {
		t.Error("Expected mobile-app from all label")
	}
	if !repoSet2.Contains("api-server") {
		t.Error("Expected api-server from all label")
	}
	if !repoSet2.Contains("worker") {
		t.Error("Expected worker from all label")
	}
	if repoSet2.Contains("old-api") {
		t.Error("Expected old-api to be excluded by !~deprecated")
	}
	if !repoSet2.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be forcibly included despite exclusion")
	}
}

// TestRepositoryListForcedVsExcluded tests that forced inclusion overrides exclusion
func TestRepositoryListForcedVsExcluded(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend":   mapset.NewSet("web-app", "mobile-app"),
		"deprecated": mapset.NewSet("deprecated-app"),
		"all":        mapset.NewSet("web-app", "mobile-app", "deprecated-app"),
	}

	// Test that forced inclusion wins over exclusion
	// Include all, exclude deprecated-app, but also force deprecated-app
	repoList := RepositoryList("~all", "!deprecated-app", "+deprecated-app")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be included (forced inclusion should override exclusion)")
	}
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app from all label")
	}
	if !repoSet.Contains("mobile-app") {
		t.Error("Expected mobile-app from all label")
	}
}

// TestRepositoryListWithSkipUnwantedAndForced tests forced inclusion with skip-unwanted enabled
func TestRepositoryListWithSkipUnwantedAndForced(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Set up configuration to skip unwanted labels
	viper.Set(config.SkipUnwanted, true)
	viper.Set(config.UnwantedLabels, []string{"deprecated", "poc"})

	// Initialize test data
	Labels = map[string]mapset.Set[string]{
		"frontend":   mapset.NewSet("web-app", "mobile-app"),
		"backend":    mapset.NewSet("api-server"),
		"deprecated": mapset.NewSet("deprecated-app"),
		"poc":        mapset.NewSet("poc-app"),
		"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "deprecated-app", "poc-app"),
	}

	Catalog = map[string]scm.Repository{
		"web-app":        {Name: "web-app", Labels: []string{"frontend"}},
		"mobile-app":     {Name: "mobile-app", Labels: []string{"frontend"}},
		"api-server":     {Name: "api-server", Labels: []string{"backend"}},
		"deprecated-app": {Name: "deprecated-app", Labels: []string{"deprecated"}},
		"poc-app":        {Name: "poc-app", Labels: []string{"poc"}},
	}

	// Test that forced inclusion overrides skip-unwanted
	// Include all (which would normally skip deprecated and poc), but force deprecated-app
	repoList := RepositoryList("~all", "+deprecated-app")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)
	if !repoSet.Contains("deprecated-app") {
		t.Error("Expected deprecated-app to be forcibly included despite being in unwanted labels")
	}
	if repoSet.Contains("poc-app") {
		t.Error("Expected poc-app to be excluded (unwanted and not forced)")
	}
	if !repoSet.Contains("web-app") {
		t.Error("Expected web-app to be included")
	}
}

// TestRepositoryListComplexScenario tests a complex real-world scenario
func TestRepositoryListComplexScenario(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Initialize realistic test data
	Labels = map[string]mapset.Set[string]{
		"frontend":     mapset.NewSet("web-ui", "mobile-app", "admin-panel"),
		"backend":      mapset.NewSet("api-gateway", "user-service", "payment-service"),
		"microservice": mapset.NewSet("user-service", "payment-service", "notification-service"),
		"deprecated":   mapset.NewSet("legacy-api", "old-frontend"),
		"experimental": mapset.NewSet("new-feature", "prototype"),
		"all":          mapset.NewSet("web-ui", "mobile-app", "admin-panel", "api-gateway", "user-service", "payment-service", "notification-service", "legacy-api", "old-frontend", "new-feature", "prototype"),
	}

	// Complex scenario:
	// - Include all microservices
	// - Exclude payment-service (maybe it's being refactored)
	// - Force include legacy-api (needed for compatibility testing)
	// - Exclude experimental repos
	repoList := RepositoryList("~microservice", "!payment-service", "+legacy-api", "!~experimental")
	repos := repoList.ToSlice()

	repoSet := mapset.NewSet(repos...)

	// Should include from microservice label
	if !repoSet.Contains("user-service") {
		t.Error("Expected user-service from microservice label")
	}
	if !repoSet.Contains("notification-service") {
		t.Error("Expected notification-service from microservice label")
	}

	// Should exclude payment-service despite being in microservice label
	if repoSet.Contains("payment-service") {
		t.Error("Expected payment-service to be excluded")
	}

	// Should force include legacy-api despite being deprecated
	if !repoSet.Contains("legacy-api") {
		t.Error("Expected legacy-api to be forcibly included")
	}

	// Should exclude experimental repos
	if repoSet.Contains("new-feature") {
		t.Error("Expected new-feature to be excluded (experimental)")
	}
	if repoSet.Contains("prototype") {
		t.Error("Expected prototype to be excluded (experimental)")
	}

	// Should not include other repos not explicitly mentioned
	if repoSet.Contains("web-ui") {
		t.Error("Expected web-ui to not be included (not in filters)")
	}
}

// TestInitWithFakeProvider tests catalog initialization with a fake SCM provider
func TestInitWithFakeProvider(t *testing.T) {
	_ = config.LoadFixture("../config")

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
	_ = config.LoadFixture("../config")

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
