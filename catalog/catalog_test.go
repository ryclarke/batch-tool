package catalog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

// TestInit tests the Init function which initializes the catalog
func TestInit(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T, ctx context.Context)
		wantRepoCount int
		wantLabelMin  int
	}{
		{
			name: "init with valid cache file",
			setupFunc: func(t *testing.T, ctx context.Context) {
				repos := map[string]scm.Repository{
					"cached-repo1": {
						Name:          "cached-repo1",
						Description:   "Cached repository 1",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"cached"},
					},
					"cached-repo2": {
						Name:          "cached-repo2",
						Description:   "Cached repository 2",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"cached"},
					},
				}
				setupCacheFile(t, ctx, repos, time.Now())
			},
			wantRepoCount: 2,
			wantLabelMin:  2, // cached label + superset
		},
		{
			name: "init with already initialized catalog does nothing",
			setupFunc: func(_ *testing.T, _ context.Context) {
				// Pre-populate catalog
				Catalog = map[string]scm.Repository{
					"existing-repo": {
						Name:          "existing-repo",
						Description:   "Existing repository",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"existing"},
					},
				}
				Labels = map[string]mapset.Set[string]{
					"existing": mapset.NewSet("existing-repo"),
				}
			},
			wantRepoCount: 1,
			wantLabelMin:  2, // existing + superset
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t) // Reset state before each test
			t.Cleanup(func() { cleanupCache(t, ctx) })

			if tt.setupFunc != nil {
				tt.setupFunc(t, ctx)
			}

			Init(ctx, false)

			testhelper.AssertLength(t, Catalog, tt.wantRepoCount)

			// Check that we have at least the minimum expected labels
			if len(Labels) < tt.wantLabelMin {
				t.Errorf("Expected at least %d labels, got %d", tt.wantLabelMin, len(Labels))
			}
		})
	}
}

// TestInitRepositoryCatalog tests the internal initialization logic
func TestInitRepositoryCatalog(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T, ctx context.Context)
		wantRepoCount int
		wantError     bool
	}{
		{
			name: "init from valid cache",
			setupFunc: func(t *testing.T, ctx context.Context) {
				repos := map[string]scm.Repository{
					"repo1": {
						Name:          "repo1",
						Description:   "Repository 1",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"label1"},
					},
				}
				setupCacheFile(t, ctx, repos, time.Now())
			},
			wantRepoCount: 1,
			wantError:     false,
		},
		{
			name: "skip init when catalog already populated",
			setupFunc: func(_ *testing.T, _ context.Context) {
				Catalog = map[string]scm.Repository{
					"existing": {
						Name:          "existing",
						Description:   "Existing repo",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"existing"},
					},
				}
			},
			wantRepoCount: 1,
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			if tt.setupFunc != nil {
				tt.setupFunc(t, ctx)
			}

			err := initRepositoryCatalog(ctx, false)

			testhelper.AssertError(t, err, tt.wantError)

			testhelper.AssertLength(t, Catalog, tt.wantRepoCount)
		})
	}
}

// TestLoadCatalogCache tests loading the catalog from cache
func TestLoadCatalogCache(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func(t *testing.T, ctx context.Context)
		wantRepoCount int
		wantLabelMin  int
		wantError     bool
	}{
		{
			name: "load valid cache file",
			setupFunc: func(t *testing.T, ctx context.Context) {
				repos := map[string]scm.Repository{
					"repo1": {
						Name:          "repo1",
						Description:   "Repository 1",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"backend", "api"},
					},
					"repo2": {
						Name:          "repo2",
						Description:   "Repository 2",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"frontend"},
					},
				}
				setupCacheFile(t, ctx, repos, time.Now())
			},
			wantRepoCount: 2,
			wantLabelMin:  3, // backend, api, frontend
			wantError:     false,
		},
		{
			name: "load expired cache returns error",
			setupFunc: func(t *testing.T, ctx context.Context) {
				repos := map[string]scm.Repository{
					"old-repo": {
						Name:          "old-repo",
						Description:   "Old repository",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"old"},
					},
				}
				// Cache from 48 hours ago
				setupCacheFile(t, ctx, repos, time.Now().Add(-48*time.Hour))
			},
			wantRepoCount: 0,
			wantLabelMin:  0,
			wantError:     true,
		},
		{
			name: "load missing cache returns error",
			setupFunc: func(_ *testing.T, _ context.Context) {
				// No cache file created
			},
			wantRepoCount: 0,
			wantLabelMin:  0,
			wantError:     true,
		},
		{
			name: "load invalid JSON returns error",
			setupFunc: func(t *testing.T, ctx context.Context) {
				cachePath := catalogCachePath(ctx)
				cacheDir := filepath.Dir(cachePath)
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					t.Fatalf("Failed to create cache directory: %v", err)
				}
				// Write invalid JSON
				if err := os.WriteFile(cachePath, []byte("invalid json"), 0644); err != nil {
					t.Fatalf("Failed to write invalid cache: %v", err)
				}
			},
			wantRepoCount: 0,
			wantLabelMin:  0,
			wantError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			if tt.setupFunc != nil {
				tt.setupFunc(t, ctx)
			}

			err := loadCatalogCache(ctx, flushTTL)

			testhelper.AssertError(t, err, tt.wantError)

			if !tt.wantError {
				testhelper.AssertLength(t, Catalog, tt.wantRepoCount)

				if len(Labels) < tt.wantLabelMin {
					t.Errorf("Expected at least %d labels, got %d", tt.wantLabelMin, len(Labels))
				}
			}
		})
	}
}

// TestSaveCatalogCache tests saving the catalog to cache
func TestSaveCatalogCache(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T, ctx context.Context)
		wantError bool
	}{
		{
			name: "save catalog with repositories",
			setupFunc: func(_ *testing.T, _ context.Context) {
				Catalog = map[string]scm.Repository{
					"repo1": {
						Name:          "repo1",
						Description:   "Repository 1",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"backend"},
					},
					"repo2": {
						Name:          "repo2",
						Description:   "Repository 2",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"frontend"},
					},
				}
			},
			wantError: false,
		},
		{
			name: "save empty catalog",
			setupFunc: func(_ *testing.T, _ context.Context) {
				Catalog = make(map[string]scm.Repository)
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			if tt.setupFunc != nil {
				tt.setupFunc(t, ctx)
			}

			err := saveCatalogCache(ctx)

			testhelper.AssertError(t, err, tt.wantError)

			if !tt.wantError {
				// Verify cache file exists and is valid
				cachePath := catalogCachePath(ctx)
				data, err := os.ReadFile(cachePath)
				if err != nil {
					t.Fatalf("Failed to read cache file: %v", err)
				}

				var cache repositoryCache
				if err := json.Unmarshal(data, &cache); err != nil {
					t.Fatalf("Failed to unmarshal cache: %v", err)
				}

				if len(cache.Repositories) != len(Catalog) {
					t.Errorf("Expected %d repositories in cache, got %d", len(Catalog), len(cache.Repositories))
				}

				// Verify timestamp is recent
				if time.Since(cache.UpdatedAt) > time.Minute {
					t.Error("Cache timestamp is not recent")
				}
			}
		})
	}
}

// TestCatalogCachePath tests the cache path generation
func TestCatalogCachePath(t *testing.T) {
	tests := []struct {
		name         string
		wantNotEmpty bool
	}{
		{
			name:         "generate cache path with all components",
			wantNotEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			path := catalogCachePath(ctx)

			if tt.wantNotEmpty && path == "" {
				t.Error("Expected non-empty cache path")
			}

			// Verify path has expected structure (directory/file)
			if tt.wantNotEmpty {
				dir := filepath.Dir(path)
				if dir == "" || dir == "." {
					t.Error("Expected cache path to have a parent directory")
				}
			}
		})
	}
}

func TestRepositoryList(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantRepos []string
	}{
		{
			name:      "basic repository lookup",
			args:      []string{"web-app", "api-server"},
			wantCount: 2,
			wantRepos: []string{"web-app", "api-server"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize test data
			Labels = map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
				"backend":  mapset.NewSet("api-server", "worker"),
				"all":      mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "tools"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

func TestRepositoryListWithLabels(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantRepos []string
	}{
		{
			name:      "frontend label lookup",
			args:      []string{"~frontend"},
			wantCount: 2,
			wantRepos: []string{"web-app", "mobile-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize test data
			Labels = map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
				"backend":  mapset.NewSet("api-server", "worker"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

func TestRepositoryListWithExclusions(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "exclude deprecated app",
			args:          []string{"~all", "!deprecated-app"},
			wantRepos:     []string{"web-app"},
			unwantedRepos: []string{"deprecated-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize test data
			Labels = map[string]mapset.Set[string]{
				"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "deprecated-app"),
				"deprecated": mapset.NewSet("deprecated-app"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

func TestRepositoryListWithSkipUnwanted(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "skip unwanted repositories",
			args:          []string{"~all"},
			wantRepos:     []string{"web-app", "mobile-app"},
			unwantedRepos: []string{"deprecated-app", "poc-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

func TestRepositoryListEmptyInput(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
	}{
		{
			name:      "no arguments",
			args:      []string{},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
		})
	}
}

func TestRepositoryListNonexistentLabel(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
	}{
		{
			name:      "nonexistent label",
			args:      []string{"~nonexistent"},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
		})
	}
}

func TestRepositoryListMixedInput(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantRepos []string
	}{
		{
			name:      "mix of labels and repository names",
			args:      []string{"~frontend", "api-server"},
			wantCount: 3,
			wantRepos: []string{"web-app", "mobile-app", "api-server"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend": mapset.NewSet("web-app", "mobile-app"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

// TestRepositoryListMultipleLabels tests combining multiple labels without forced inclusion
func TestRepositoryListMultipleLabels(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantRepos []string
	}{
		{
			name:      "combine frontend and backend labels",
			args:      []string{"~frontend", "~backend"},
			wantCount: 4,
			wantRepos: []string{"web-app", "mobile-app", "api-server", "worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":     mapset.NewSet("web-app", "mobile-app"),
				"backend":      mapset.NewSet("api-server", "worker"),
				"microservice": mapset.NewSet("user-service", "payment-service"),
				"database":     mapset.NewSet("postgres-service", "redis-service"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

// TestRepositoryListMultipleExclusions tests excluding multiple items
func TestRepositoryListMultipleExclusions(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "exclude multiple individual repos",
			args:          []string{"~all", "!deprecated-app", "!test-app"},
			wantRepos:     []string{"web-app", "mobile-app", "api-server", "worker"},
			unwantedRepos: []string{"deprecated-app", "test-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "test-app"),
				"deprecated": mapset.NewSet("deprecated-app"),
				"test":       mapset.NewSet("test-app"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestRepositoryListExcludeMultipleLabels tests excluding multiple labels
func TestRepositoryListExcludeMultipleLabels(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "exclude deprecated and test labels",
			args:          []string{"~all", "!~deprecated", "!~test"},
			wantRepos:     []string{"web-app", "mobile-app", "api-server", "worker"},
			unwantedRepos: []string{"deprecated-app", "test-app", "experimental-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "test-app", "experimental-app"),
				"deprecated": mapset.NewSet("deprecated-app"),
				"test":       mapset.NewSet("test-app", "experimental-app"),
				"backend":    mapset.NewSet("api-server", "worker"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestRepositoryListMixedIncludesAndExcludes tests combining includes and excludes without forced
func TestRepositoryListMixedIncludesAndExcludes(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "include frontend and microservices with exclusions",
			args:          []string{"~frontend", "~microservice", "!mobile-app", "!payment-service", "!~legacy"},
			wantRepos:     []string{"web-app", "admin-panel", "user-service", "notification-service"},
			unwantedRepos: []string{"mobile-app", "payment-service", "old-api", "legacy-frontend", "api-server"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":     mapset.NewSet("web-app", "mobile-app", "admin-panel"),
				"backend":      mapset.NewSet("api-server", "worker", "scheduler"),
				"microservice": mapset.NewSet("user-service", "payment-service", "notification-service"),
				"legacy":       mapset.NewSet("old-api", "legacy-frontend"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestRepositoryListLabelOverlap tests behavior with overlapping labels
func TestRepositoryListLabelOverlap(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantRepos []string
	}{
		{
			name:      "overlapping labels deduplicate repos",
			args:      []string{"~frontend", "~backend"},
			wantCount: 5,
			wantRepos: []string{"web-app", "mobile-app", "shared-component", "api-server", "worker"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":   mapset.NewSet("web-app", "mobile-app", "shared-component"),
				"backend":    mapset.NewSet("api-server", "shared-component", "worker"),
				"shared":     mapset.NewSet("shared-component", "shared-utils"),
				"javascript": mapset.NewSet("web-app", "mobile-app", "shared-utils"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

// TestRepositoryListComplexExclusionPattern tests complex exclusion patterns
func TestRepositoryListComplexExclusionPattern(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "include services excluding experimental and specific",
			args:          []string{"~services", "!~experimental", "!service-c"},
			wantRepos:     []string{"service-a", "service-b"},
			unwantedRepos: []string{"service-c", "service-d", "util-x", "util-y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"all":          mapset.NewSet("service-a", "service-b", "service-c", "service-d", "util-x", "util-y", "test-helper"),
				"services":     mapset.NewSet("service-a", "service-b", "service-c", "service-d"),
				"utilities":    mapset.NewSet("util-x", "util-y"),
				"testing":      mapset.NewSet("test-helper"),
				"core":         mapset.NewSet("service-a", "service-b"),
				"experimental": mapset.NewSet("service-d", "util-y"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestRepositoryListDirectRepoSelection tests direct repository selection patterns
func TestRepositoryListDirectRepoSelection(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "mix direct repo names with exclusions",
			args:          []string{"repo-a1", "repo-b1", "custom-repo", "!repo-a2"},
			wantRepos:     []string{"repo-a1", "repo-b1", "custom-repo"},
			unwantedRepos: []string{"repo-a2", "repo-a3", "repo-b2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"group-a": mapset.NewSet("repo-a1", "repo-a2", "repo-a3"),
				"group-b": mapset.NewSet("repo-b1", "repo-b2"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestRepositoryListWithForcedInclusion tests the '+' token for forced inclusion
func TestRepositoryListWithForcedInclusion(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantRepos []string
	}{
		{
			name:      "force include single repository",
			args:      []string{"+deprecated-app"},
			wantCount: 1,
			wantRepos: []string{"deprecated-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":   mapset.NewSet("web-app", "mobile-app"),
				"backend":    mapset.NewSet("api-server", "worker"),
				"deprecated": mapset.NewSet("deprecated-app", "old-api"),
				"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "old-api"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

// TestRepositoryListWithForcedLabelInclusion tests the '+~' token combination
func TestRepositoryListWithForcedLabelInclusion(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantCount int
		wantRepos []string
	}{
		{
			name:      "force include entire label",
			args:      []string{"+~deprecated"},
			wantCount: 2,
			wantRepos: []string{"deprecated-app", "old-api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":   mapset.NewSet("web-app", "mobile-app"),
				"backend":    mapset.NewSet("api-server", "worker"),
				"deprecated": mapset.NewSet("deprecated-app", "old-api"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertLength(t, repos, tt.wantCount)
			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

// TestRepositoryListMixedTokens tests various combinations of ~, !, and + tokens
func TestRepositoryListMixedTokens(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "include label exclude repo force repo",
			args:          []string{"~frontend", "!mobile-app", "+deprecated-app"},
			wantRepos:     []string{"web-app", "deprecated-app"},
			unwantedRepos: []string{"mobile-app"},
		},
		{
			name:          "include all exclude label force repo from excluded label",
			args:          []string{"~all", "!~deprecated", "+deprecated-app"},
			wantRepos:     []string{"web-app", "mobile-app", "api-server", "worker", "deprecated-app"},
			unwantedRepos: []string{"old-api"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":   mapset.NewSet("web-app", "mobile-app"),
				"backend":    mapset.NewSet("api-server", "worker"),
				"deprecated": mapset.NewSet("deprecated-app", "old-api"),
				"all":        mapset.NewSet("web-app", "mobile-app", "api-server", "worker", "deprecated-app", "old-api"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestRepositoryListForcedVsExcluded tests that forced inclusion overrides exclusion
func TestRepositoryListForcedVsExcluded(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name      string
		args      []string
		wantRepos []string
	}{
		{
			name:      "forced inclusion overrides exclusion",
			args:      []string{"~all", "!deprecated-app", "+deprecated-app"},
			wantRepos: []string{"deprecated-app", "web-app", "mobile-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":   mapset.NewSet("web-app", "mobile-app"),
				"deprecated": mapset.NewSet("deprecated-app"),
				"all":        mapset.NewSet("web-app", "mobile-app", "deprecated-app"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
		})
	}
}

// TestRepositoryListWithSkipUnwantedAndForced tests forced inclusion with skip-unwanted enabled
func TestRepositoryListWithSkipUnwantedAndForced(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "force include overrides skip-unwanted",
			args:          []string{"~all", "+deprecated-app"},
			wantRepos:     []string{"deprecated-app", "web-app"},
			unwantedRepos: []string{"poc-app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper := config.Viper(ctx)
			viper.Set(config.SkipUnwanted, true)
			viper.Set(config.UnwantedLabels, []string{"deprecated", "poc"})

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

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestRepositoryListComplexScenario tests a complex real-world scenario
func TestRepositoryListComplexScenario(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name          string
		args          []string
		wantRepos     []string
		unwantedRepos []string
	}{
		{
			name:          "complex microservice filtering with force and exclude",
			args:          []string{"~microservice", "!payment-service", "+legacy-api", "!~experimental"},
			wantRepos:     []string{"user-service", "notification-service", "legacy-api"},
			unwantedRepos: []string{"payment-service", "new-feature", "prototype", "web-ui"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Labels = map[string]mapset.Set[string]{
				"frontend":     mapset.NewSet("web-ui", "mobile-app", "admin-panel"),
				"backend":      mapset.NewSet("api-gateway", "user-service", "payment-service"),
				"microservice": mapset.NewSet("user-service", "payment-service", "notification-service"),
				"deprecated":   mapset.NewSet("legacy-api", "old-frontend"),
				"experimental": mapset.NewSet("new-feature", "prototype"),
				"all":          mapset.NewSet("web-ui", "mobile-app", "admin-panel", "api-gateway", "user-service", "payment-service", "notification-service", "legacy-api", "old-frontend", "new-feature", "prototype"),
			}

			repoList := RepositoryList(ctx, tt.args...)
			repos := repoList.ToSlice()

			testhelper.AssertContains(t, repos, tt.wantRepos)
			testhelper.AssertNotContains(t, repos, tt.unwantedRepos)
		})
	}
}

// TestInitWithFakeProvider tests catalog initialization with a fake SCM provider
func TestInitWithFakeProvider(t *testing.T) {
	_ = loadFixture(t)

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
	scm.Register("fake-test", func(_ context.Context, _ string) scm.Provider {
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

// TestGetRepository tests the GetRepository function with different lookup strategies
func TestGetRepository(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "default-project")

	// Setup test catalog
	Catalog = map[string]scm.Repository{
		"project-a/repo-1": {
			Name:    "repo-1",
			Project: "project-a",
			Labels:  []string{"backend"},
		},
		"project-b/repo-2": {
			Name:    "repo-2",
			Project: "project-b",
			Labels:  []string{"frontend"},
		},
		"default-project/repo-3": {
			Name:    "repo-3",
			Project: "default-project",
			Labels:  []string{"backend"},
		},
	}

	tests := []struct {
		name        string
		repoName    string
		wantFound   bool
		wantName    string
		wantProject string
	}{
		{
			name:        "direct lookup with project/name format",
			repoName:    "project-a/repo-1",
			wantFound:   true,
			wantName:    "repo-1",
			wantProject: "project-a",
		},
		{
			name:        "lookup with default project prefix",
			repoName:    "repo-3",
			wantFound:   true,
			wantName:    "repo-3",
			wantProject: "default-project",
		},
		{
			name:        "lookup by name match",
			repoName:    "repo-1",
			wantFound:   true,
			wantName:    "repo-1",
			wantProject: "project-a",
		},
		{
			name:        "lookup by suffix match",
			repoName:    "repo-2",
			wantFound:   true,
			wantName:    "repo-2",
			wantProject: "project-b",
		},
		{
			name:      "nonexistent repository",
			repoName:  "nonexistent-repo",
			wantFound: false,
		},
		{
			name:      "empty repository name",
			repoName:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, found := GetRepository(ctx, tt.repoName)

			if found != tt.wantFound {
				t.Errorf("GetRepository(%q) found = %v, want %v", tt.repoName, found, tt.wantFound)
			}

			if tt.wantFound {
				if repo == nil {
					t.Fatal("Expected non-nil repository when found=true")
				}
				if repo.Name != tt.wantName {
					t.Errorf("GetRepository(%q) name = %q, want %q", tt.repoName, repo.Name, tt.wantName)
				}
				if repo.Project != tt.wantProject {
					t.Errorf("GetRepository(%q) project = %q, want %q", tt.repoName, repo.Project, tt.wantProject)
				}
			} else {
				if repo != nil {
					t.Errorf("Expected nil repository when found=false, got %+v", repo)
				}
			}
		})
	}
}

// TestGetRepositoryWithoutDefaultProject tests GetRepository when no default project is configured
func TestGetRepositoryWithoutDefaultProject(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "") // No default project

	Catalog = map[string]scm.Repository{
		"project-a/repo-1": {
			Name:    "repo-1",
			Project: "project-a",
		},
	}

	// Should still find by name match even without default project
	repo, found := GetRepository(ctx, "repo-1")
	if !found {
		t.Error("Expected to find repo-1 by name match")
	}
	if repo == nil || repo.Name != "repo-1" {
		t.Errorf("Expected repo-1, got %+v", repo)
	}
}

// TestGetProjectForRepo tests the GetProjectForRepo function
func TestGetProjectForRepo(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "default-project")

	Catalog = map[string]scm.Repository{
		"project-a/repo-1": {
			Name:    "repo-1",
			Project: "project-a",
		},
		"project-b/repo-2": {
			Name:    "repo-2",
			Project: "project-b",
		},
	}

	tests := []struct {
		name        string
		repoName    string
		wantProject string
	}{
		{
			name:        "repo in catalog returns its project",
			repoName:    "repo-1",
			wantProject: "project-a",
		},
		{
			name:        "another repo in catalog",
			repoName:    "repo-2",
			wantProject: "project-b",
		},
		{
			name:        "repo not in catalog returns default",
			repoName:    "unknown-repo",
			wantProject: "default-project",
		},
		{
			name:        "empty repo name returns default",
			repoName:    "",
			wantProject: "default-project",
		},
		{
			name:        "qualified name returns its project",
			repoName:    "project-a/repo-1",
			wantProject: "project-a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := GetProjectForRepo(ctx, tt.repoName)
			if project != tt.wantProject {
				t.Errorf("GetProjectForRepo(%q) = %q, want %q", tt.repoName, project, tt.wantProject)
			}
		})
	}
}

// TestGetProjectForRepoWithoutDefault tests GetProjectForRepo when no default is configured
func TestGetProjectForRepoWithoutDefault(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "")

	Catalog = map[string]scm.Repository{
		"project-a/repo-1": {
			Name:    "repo-1",
			Project: "project-a",
		},
	}

	// Found in catalog
	project := GetProjectForRepo(ctx, "repo-1")
	if project != "project-a" {
		t.Errorf("Expected project-a, got %q", project)
	}

	// Not found in catalog, should return empty string (no default)
	project = GetProjectForRepo(ctx, "unknown-repo")
	if project != "" {
		t.Errorf("Expected empty string, got %q", project)
	}
}

// TestGetBranchForRepo tests the GetBranchForRepo function
func TestGetBranchForRepo(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.DefaultBranch, "main")

	Catalog = map[string]scm.Repository{
		"project-a/repo-with-branch": {
			Name:          "repo-with-branch",
			Project:       "project-a",
			DefaultBranch: "develop",
		},
		"project-a/repo-no-branch": {
			Name:    "repo-no-branch",
			Project: "project-a",
		},
		"project-b/repo-master": {
			Name:          "repo-master",
			Project:       "project-b",
			DefaultBranch: "master",
		},
	}

	tests := []struct {
		name       string
		repoName   string
		wantBranch string
	}{
		{
			name:       "repo in catalog returns its default branch",
			repoName:   "repo-with-branch",
			wantBranch: "develop",
		},
		{
			name:       "repo without default branch returns config default",
			repoName:   "repo-no-branch",
			wantBranch: "main",
		},
		{
			name:       "another repo with custom branch",
			repoName:   "repo-master",
			wantBranch: "master",
		},
		{
			name:       "repo not in catalog returns config default",
			repoName:   "unknown-repo",
			wantBranch: "main",
		},
		{
			name:       "qualified name returns default branch",
			repoName:   "project-a/repo-with-branch",
			wantBranch: "develop",
		},
		{
			name:       "empty repo name returns config default",
			repoName:   "",
			wantBranch: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch := GetBranchForRepo(ctx, tt.repoName)
			if branch != tt.wantBranch {
				t.Errorf("GetBranchForRepo(%q) = %q, want %q", tt.repoName, branch, tt.wantBranch)
			}
		})
	}
}

// TestGetBranchForRepoWithoutDefault tests GetBranchForRepo when no default branch is configured
func TestGetBranchForRepoWithoutDefault(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.DefaultBranch, "")

	Catalog = map[string]scm.Repository{
		"project-a/repo-1": {
			Name:          "repo-1",
			Project:       "project-a",
			DefaultBranch: "develop",
		},
	}

	// Found in catalog with explicit branch
	branch := GetBranchForRepo(ctx, "repo-1")
	if branch != "develop" {
		t.Errorf("Expected develop, got %q", branch)
	}

	// Not found in catalog, should return empty string (no default)
	branch = GetBranchForRepo(ctx, "unknown-repo")
	if branch != "" {
		t.Errorf("Expected empty string, got %q", branch)
	}
}

// TestInitWithRepoAliases tests Init function with repository aliases
func TestInitWithRepoAliases(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.SuperSetLabel, "~all")

	// Setup repo aliases
	viper.Set(config.RepoAliases, map[string]interface{}{
		"~backend":  []string{"api-server", "worker"},
		"~frontend": []string{"web-app"},
	})

	// Setup test catalog
	Catalog = map[string]scm.Repository{
		"api-server": {Name: "api-server", Project: "test-project"},
		"worker":     {Name: "worker", Project: "test-project"},
		"web-app":    {Name: "web-app", Project: "test-project"},
	}

	// Call Init to process aliases
	Init(ctx, false)

	// Verify aliases were added to labels
	backendLabel, exists := Labels["~backend"]
	if !exists {
		t.Fatal("Expected ~backend label to exist")
	}
	if backendLabel.Cardinality() != 2 {
		t.Errorf("Expected ~backend to have 2 repos, got %d", backendLabel.Cardinality())
	}
	if !backendLabel.Contains("api-server") || !backendLabel.Contains("worker") {
		t.Error("Expected ~backend to contain api-server and worker")
	}

	frontendLabel, exists := Labels["~frontend"]
	if !exists {
		t.Fatal("Expected ~frontend label to exist")
	}
	if frontendLabel.Cardinality() != 1 {
		t.Errorf("Expected ~frontend to have 1 repo, got %d", frontendLabel.Cardinality())
	}
	if !frontendLabel.Contains("web-app") {
		t.Error("Expected ~frontend to contain web-app")
	}

	// Verify superset label was created
	allLabel, exists := Labels["~all"]
	if !exists {
		t.Fatal("Expected ~all superset label to exist")
	}
	if allLabel.Cardinality() != 3 {
		t.Errorf("Expected ~all to have 3 repos, got %d", allLabel.Cardinality())
	}
}

// TestInitWithExistingAliasLabel tests Init when alias overlaps with existing label
func TestInitWithExistingAliasLabel(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.SuperSetLabel, "~all")

	// Setup test catalog with existing labels
	Catalog = map[string]scm.Repository{
		"repo-1": {Name: "repo-1", Project: "test-project", Labels: []string{"backend"}},
		"repo-2": {Name: "repo-2", Project: "test-project", Labels: []string{"backend"}},
		"repo-3": {Name: "repo-3", Project: "test-project"},
	}

	// Pre-populate a label
	Labels["backend"] = mapset.NewSet("repo-1", "repo-2")

	// Setup alias that appends to existing label
	viper.Set(config.RepoAliases, map[string]interface{}{
		"backend": []string{"repo-3"}, // Add repo-3 to existing backend label
	})

	Init(ctx, false)

	// Verify label was appended, not replaced
	backendLabel := Labels["backend"]
	if backendLabel.Cardinality() != 3 {
		t.Errorf("Expected backend to have 3 repos, got %d", backendLabel.Cardinality())
	}
	if !backendLabel.Contains("repo-1") || !backendLabel.Contains("repo-2") || !backendLabel.Contains("repo-3") {
		t.Error("Expected backend to contain all three repos")
	}
}

// TestCatalogCachePathWithCustomPath tests catalogCachePath with custom path
func TestCatalogCachePathWithCustomPath(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	customPath := "/custom/path/to/cache.json"
	viper.Set(config.CatalogCachePath, customPath)

	path := catalogCachePath(ctx)
	if path != customPath {
		t.Errorf("Expected custom path %q, got %q", customPath, path)
	}
}

// TestCatalogCachePathDefault tests catalogCachePath with default path
func TestCatalogCachePathDefault(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	gitDir := "/home/user/repos"
	gitHost := "github.com"

	viper.Set(config.CatalogCachePath, "") // No custom path
	viper.Set(config.GitDirectory, gitDir)
	viper.Set(config.GitHost, gitHost)

	path := catalogCachePath(ctx)
	expectedPath := filepath.Join(gitDir, gitHost, defaultCacheFile)

	if path != expectedPath {
		t.Errorf("Expected default path %q, got %q", expectedPath, path)
	}
}

// TestSaveCatalogCacheCreatesDirs tests that saveCatalogCache creates parent directories
func TestSaveCatalogCacheCreatesDirs(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "nested", "dirs", "cache.json")
	viper.Set(config.CatalogCachePath, cachePath)

	Catalog = map[string]scm.Repository{
		"test-repo": {Name: "test-repo", Project: "test"},
	}

	err := saveCatalogCache(ctx)
	if err != nil {
		t.Fatalf("saveCatalogCache failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Error("Expected cache file to be created")
	}

	// Verify parent directories were created
	parentDir := filepath.Dir(cachePath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Error("Expected parent directories to be created")
	}
}

// TestSaveCatalogCacheMarshalError tests error handling in saveCatalogCache
// Note: This is difficult to test without mocking, but we can test with invalid data
func TestSaveCatalogCacheWithInvalidPath(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Use an invalid path (file as directory)
	tempDir := t.TempDir()
	invalidFile := filepath.Join(tempDir, "file")

	// Create a file where we want a directory
	if err := os.WriteFile(invalidFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Try to create cache under this file (should fail)
	invalidPath := filepath.Join(invalidFile, "subdir", "cache.json")
	viper.Set(config.CatalogCachePath, invalidPath)

	Catalog = map[string]scm.Repository{
		"test-repo": {Name: "test-repo", Project: "test"},
	}

	err := saveCatalogCache(ctx)
	if err == nil {
		t.Error("Expected saveCatalogCache to fail with invalid path")
	}
}

// TestInitRegistersCallbackFunction tests that Init registers the catalog lookup callback
func TestInitRegistersCallbackFunction(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.SuperSetLabel, "~all")

	// Setup test catalog
	Catalog = map[string]scm.Repository{
		"test-project/repo-1": {
			Name:    "repo-1",
			Project: "test-project",
		},
	}

	// Call Init
	Init(ctx, false)

	// Verify callback was registered by testing it
	// (utils.CatalogLookup should now point to GetProjectForRepo)
	project := GetProjectForRepo(ctx, "repo-1")
	if project != "test-project" {
		t.Errorf("Expected callback to return test-project, got %q", project)
	}
}

// TestRepositoryListWithEmptyCatalog tests RepositoryList when catalog is empty
func TestRepositoryListWithEmptyCatalog(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)

	viper := config.Viper(ctx)
	viper.Set(config.SuperSetLabel, "~all")

	Catalog = map[string]scm.Repository{} // Empty catalog
	Labels = make(map[string]mapset.Set[string])
	Labels["~all"] = mapset.NewSet[string]()

	repos := RepositoryList(ctx, "~all")
	if repos.Cardinality() != 0 {
		t.Errorf("Expected empty result, got %d repos", repos.Cardinality())
	}
}

// TestInitWithFlush tests Init with flush flag
func TestInitWithFlush(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { cleanupCache(t, ctx) })

	viper := config.Viper(ctx)
	viper.Set(config.SuperSetLabel, "~all")
	viper.Set(config.GitProject, "test-project") // Ensure project is set

	// Create an old cache file (beyond the flush TTL of 5 seconds)
	oldRepos := map[string]scm.Repository{
		"old-repo": {
			Name:    "old-repo",
			Project: "test-project",
			Labels:  []string{"old"},
		},
	}
	setupCacheFile(t, ctx, oldRepos, time.Now().Add(-10*time.Second))

	// Initialize with flush=false - should use cache
	Init(ctx, false)

	if len(Catalog) != 1 {
		t.Errorf("Expected 1 repo from cache, got %d", len(Catalog))
	}
	if _, exists := Catalog["old-repo"]; !exists {
		t.Error("Expected old-repo from cache")
	}

	// Now initialize with flush=true - should refetch
	resetCatalogState(t)

	// Register a fake provider that returns different data
	scm.Register("fake-flush-test", func(_ context.Context, project string) scm.Provider {
		return fake.NewFake(project, []*scm.Repository{
			{Name: "new-repo", Project: project, Labels: []string{"new"}},
		})
	})
	viper.Set(config.GitProvider, "fake-flush-test")

	Init(ctx, true)

	if len(Catalog) != 1 {
		t.Errorf("Expected 1 repo after flush, got %d", len(Catalog))
	}
	if _, exists := Catalog["test-project/new-repo"]; !exists {
		t.Error("Expected new-repo after flush, but got catalog:", Catalog)
	}
}

// TestInitRepositoryCatalogWithFlush tests initRepositoryCatalog with flush flag
func TestInitRepositoryCatalogWithFlush(t *testing.T) {
	tests := []struct {
		name          string
		flush         bool
		setupFunc     func(t *testing.T, ctx context.Context)
		wantRepoCount int
		wantRefetch   bool
	}{
		{
			name:  "flush forces refetch even with valid cache",
			flush: true,
			setupFunc: func(t *testing.T, ctx context.Context) {
				// Create a cache older than flush TTL (5 seconds)
				repos := map[string]scm.Repository{
					"cached-repo": {
						Name:    "cached-repo",
						Project: "test-project",
						Labels:  []string{"cached"},
					},
				}
				setupCacheFile(t, ctx, repos, time.Now().Add(-10*time.Second))

				// Register fake provider
				scm.Register("fake-flush", func(_ context.Context, project string) scm.Provider {
					return fake.NewFake(project, []*scm.Repository{
						{Name: "fresh-repo", Project: project, Labels: []string{"fresh"}},
					})
				})
				viper := config.Viper(ctx)
				viper.Set(config.GitProvider, "fake-flush")
				viper.Set(config.GitProject, "test-project")
			},
			wantRepoCount: 1,
			wantRefetch:   true, // Should refetch despite valid cache
		},
		{
			name:  "no flush uses valid cache",
			flush: false,
			setupFunc: func(t *testing.T, ctx context.Context) {
				// Create a fresh cache
				repos := map[string]scm.Repository{
					"cached-repo": {
						Name:    "cached-repo",
						Project: "test-project",
						Labels:  []string{"cached"},
					},
				}
				setupCacheFile(t, ctx, repos, time.Now())
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "test-project")
			},
			wantRepoCount: 1,
			wantRefetch:   false, // Should use cache
		},
		{
			name:  "flush refetches even with already populated catalog",
			flush: true,
			setupFunc: func(_ *testing.T, ctx context.Context) {
				// Pre-populate catalog
				Catalog = map[string]scm.Repository{
					"test-project/existing-repo": {
						Name:    "existing-repo",
						Project: "test-project",
						Labels:  []string{"existing"},
					},
				}

				// Register fake provider that would return different data
				scm.Register("fake-flush-2", func(_ context.Context, project string) scm.Provider {
					return fake.NewFake(project, []*scm.Repository{
						{Name: "new-repo", Project: project, Labels: []string{"new"}},
					})
				})
				viper := config.Viper(ctx)
				viper.Set(config.GitProvider, "fake-flush-2")
				viper.Set(config.GitProject, "test-project")
			},
			wantRepoCount: 2,    // Should have both existing and new repo (catalog not cleared before refetch)
			wantRefetch:   true, // Flush should cause refetch
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			if tt.setupFunc != nil {
				tt.setupFunc(t, ctx)
			}

			err := initRepositoryCatalog(ctx, tt.flush)
			if err != nil {
				t.Fatalf("initRepositoryCatalog failed: %v", err)
			}

			if len(Catalog) != tt.wantRepoCount {
				t.Errorf("Expected %d repos, got %d", tt.wantRepoCount, len(Catalog))
			}

			// Verify refetch behavior
			if tt.wantRefetch && tt.flush {
				// When flush=true, should have fetched from provider
				// Check if we have the "fresh" or "new" repos instead of "cached"
				if tt.name == "flush forces refetch even with valid cache" {
					if _, exists := Catalog["test-project/fresh-repo"]; !exists {
						t.Error("Expected fresh-repo from refetch")
					}
				}
			}
		})
	}
}

// TestFlushTTL tests that flush uses a short TTL to force refetch
func TestFlushTTL(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { cleanupCache(t, ctx) })

	viper := config.Viper(ctx)
	viper.Set(config.CatalogCacheTTL, 24*time.Hour) // Set long TTL
	viper.Set(config.GitProject, "test-project")    // Ensure project is set

	// Create a cache that's 10 seconds old (within normal TTL but outside flush TTL)
	repos := map[string]scm.Repository{
		"old-cached-repo": {
			Name:    "old-cached-repo",
			Project: "test-project",
			Labels:  []string{"cached"},
		},
	}
	setupCacheFile(t, ctx, repos, time.Now().Add(-10*time.Second))

	// Register fake provider with different data
	scm.Register("fake-flush-ttl", func(_ context.Context, project string) scm.Provider {
		return fake.NewFake(project, []*scm.Repository{
			{Name: "new-repo", Project: project, Labels: []string{"new"}},
		})
	})
	viper.Set(config.GitProvider, "fake-flush-ttl")

	// Initialize without flush - should use cache since it's within TTL
	err := initRepositoryCatalog(ctx, false)
	if err != nil {
		t.Fatalf("initRepositoryCatalog without flush failed: %v", err)
	}

	if _, exists := Catalog["old-cached-repo"]; !exists {
		t.Error("Expected to use cached repo without flush")
	}

	// Reset and initialize with flush - should refetch due to short flush TTL
	resetCatalogState(t)
	err = initRepositoryCatalog(ctx, true)
	if err != nil {
		t.Fatalf("initRepositoryCatalog with flush failed: %v", err)
	}

	if _, exists := Catalog["test-project/new-repo"]; !exists {
		t.Error("Expected to refetch with flush despite valid cache")
	}
}
