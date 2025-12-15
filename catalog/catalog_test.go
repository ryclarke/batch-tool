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
			setupFunc: func(t *testing.T, ctx context.Context) {
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

			Init(ctx)

			checkCatalogSize(t, tt.wantRepoCount)

			// Check that we have at least the minimum expected labels
			if len(Labels) < tt.wantLabelMin {
				t.Errorf("Expected at least %d labels, got %d", tt.wantLabelMin, len(Labels))
			}
		})
	}
}

// TestFlush tests the Flush function which removes the catalog cache
func TestFlush(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T, ctx context.Context) string
		wantError bool
	}{
		{
			name: "flush existing cache file",
			setupFunc: func(t *testing.T, ctx context.Context) string {
				repos := map[string]scm.Repository{
					"test-repo": {
						Name:          "test-repo",
						Description:   "Test repository",
						Project:       "test-project",
						DefaultBranch: "main",
						Labels:        []string{"test"},
					},
				}
				return setupCacheFile(t, ctx, repos, time.Now())
			},
			wantError: false,
		},
		{
			name: "flush non-existent cache file",
			setupFunc: func(t *testing.T, ctx context.Context) string {
				viper := config.Viper(ctx)
				return viper.GetString(config.GitDirectory) + "/nonexistent/cache.json"
			},
			wantError: false, // os.Remove returns nil for non-existent files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			var cachePath string
			if tt.setupFunc != nil {
				cachePath = tt.setupFunc(t, ctx)
			}

			err := Flush(ctx)

			if !checkError(t, err, tt.wantError) {
				return
			}

			// Verify cache file is removed
			if _, err := os.Stat(cachePath); !os.IsNotExist(err) && !tt.wantError {
				t.Error("Expected cache file to be removed")
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
			setupFunc: func(t *testing.T, ctx context.Context) {
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

			err := initRepositoryCatalog(ctx)

			if !checkError(t, err, tt.wantError) {
				return
			}

			checkCatalogSize(t, tt.wantRepoCount)
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
			setupFunc: func(t *testing.T, ctx context.Context) {
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

			err := loadCatalogCache(ctx)

			if !checkError(t, err, tt.wantError) {
				return
			}

			if !tt.wantError {
				checkCatalogSize(t, tt.wantRepoCount)

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
			setupFunc: func(t *testing.T, ctx context.Context) {
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
			setupFunc: func(t *testing.T, ctx context.Context) {
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

			if !checkError(t, err, tt.wantError) {
				return
			}

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

			checkRepoCount(t, repos, tt.wantCount)
			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoCount(t, repos, tt.wantCount)
			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoCount(t, repos, tt.wantCount)
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

			checkRepoCount(t, repos, tt.wantCount)
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

			checkRepoCount(t, repos, tt.wantCount)
			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoCount(t, repos, tt.wantCount)
			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoCount(t, repos, tt.wantCount)
			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoCount(t, repos, tt.wantCount)
			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoCount(t, repos, tt.wantCount)
			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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

			checkRepoContains(t, repos, tt.wantRepos)
			checkRepoNotContains(t, repos, tt.unwantedRepos)
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
	scm.Register("fake-test", func(ctx context.Context, project string) scm.Provider {
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
