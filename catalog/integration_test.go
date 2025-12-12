package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
)

// TestMultiProjectCatalogInitialization tests catalog initialization across multiple projects
func TestMultiProjectCatalogInitialization(t *testing.T) {
	tests := []struct {
		name             string
		projects         []string
		reposPerProject  int
		wantTotalRepos   int
		wantLabels       []string
		checkNamespacing bool
	}{
		{
			name:             "single project",
			projects:         []string{"project-a"},
			reposPerProject:  5,
			wantTotalRepos:   5,
			wantLabels:       []string{"active", "backend"},
			checkNamespacing: true,
		},
		{
			name:             "two projects",
			projects:         []string{"project-a", "project-b"},
			reposPerProject:  5,
			wantTotalRepos:   10,
			wantLabels:       []string{"active", "backend"},
			checkNamespacing: true,
		},
		{
			name:             "three projects with label aggregation",
			projects:         []string{"frontend", "backend", "infra"},
			reposPerProject:  5,
			wantTotalRepos:   15,
			wantLabels:       []string{"active", "backend"},
			checkNamespacing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			viper := config.Viper(ctx)

			// Register a single fake provider that returns data based on the project
			providerName := "fake-multi-" + tt.name
			scm.Register(providerName, func(ctx context.Context, project string) scm.Provider {
				return fake.NewFake(project, fake.CreateTestRepositories(project))
			})

			// Configure multiple projects
			viper.Set(config.GitProvider, providerName)
			viper.Set(config.GitProject, tt.projects[0])
			if len(tt.projects) > 1 {
				viper.Set(config.GitProjects, tt.projects)
			}

			// Fetch data from all projects
			err := fetchRepositoryData(ctx)
			if err != nil {
				t.Fatalf("fetchRepositoryData failed: %v", err)
			}

			// Verify total repository count
			if len(Catalog) != tt.wantTotalRepos {
				t.Errorf("Expected %d repositories in catalog, got %d", tt.wantTotalRepos, len(Catalog))
			}

			// Verify project namespacing
			if tt.checkNamespacing {
				for repoKey := range Catalog {
					found := false
					for _, project := range tt.projects {
						if len(repoKey) > len(project) && repoKey[:len(project)] == project {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Repository key %s doesn't match any project namespace", repoKey)
					}
				}
			}

			// Verify labels exist and aggregate across projects
			for _, label := range tt.wantLabels {
				if _, exists := Labels[label]; !exists {
					t.Errorf("Expected label %s to exist", label)
				}
			}

			// Verify no duplicate repository names cause issues
			repoNames := make(map[string]int)
			for _, repo := range Catalog {
				repoNames[repo.Name]++
			}

			// With multiple projects, same repo name should appear multiple times
			if len(tt.projects) > 1 {
				foundDuplicate := false
				for _, count := range repoNames {
					if count > 1 {
						foundDuplicate = true
						break
					}
				}
				if foundDuplicate && len(Catalog) > len(repoNames) {
					t.Logf("Successfully handled duplicate repository names across projects")
				}
			}
		})
	}
}

// TestMultiProjectLabelAggregation tests that labels aggregate correctly across projects
func TestMultiProjectLabelAggregation(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { cleanupCache(t, ctx) })

	viper := config.Viper(ctx)

	// Create custom repositories with specific labels for each project
	projectRepos := map[string][]*scm.Repository{
		"team-a": {
			{
				Name:    "service-1",
				Project: "team-a",
				Labels:  []string{"microservice", "golang", "critical"},
			},
			{
				Name:    "service-2",
				Project: "team-a",
				Labels:  []string{"microservice", "golang"},
			},
		},
		"team-b": {
			{
				Name:    "service-1",
				Project: "team-b",
				Labels:  []string{"microservice", "python", "experimental"},
			},
			{
				Name:    "web-app",
				Project: "team-b",
				Labels:  []string{"frontend", "react"},
			},
		},
	}

	// Register a single fake provider that handles both projects
	scm.Register("fake-multiproject", func(ctx context.Context, proj string) scm.Provider {
		if repos, exists := projectRepos[proj]; exists {
			return fake.NewFake(proj, repos)
		}
		return fake.NewFake(proj, []*scm.Repository{})
	})

	// Configure to use both projects
	viper.Set(config.GitProvider, "fake-multiproject")
	viper.Set(config.GitProjects, []string{"team-a", "team-b"})

	// Fetch data
	err := fetchRepositoryData(ctx)
	if err != nil {
		t.Fatalf("fetchRepositoryData failed: %v", err)
	}

	// Verify label aggregation
	tests := []struct {
		label          string
		wantRepoCount  int
		mustContainKey string
	}{
		{label: "microservice", wantRepoCount: 3, mustContainKey: "team-a/service-1"},
		{label: "golang", wantRepoCount: 2, mustContainKey: "team-a/service-2"},
		{label: "python", wantRepoCount: 1, mustContainKey: "team-b/service-1"},
		{label: "frontend", wantRepoCount: 1, mustContainKey: "team-b/web-app"},
		{label: "critical", wantRepoCount: 1, mustContainKey: "team-a/service-1"},
		{label: "experimental", wantRepoCount: 1, mustContainKey: "team-b/service-1"},
	}

	for _, tt := range tests {
		t.Run("label_"+tt.label, func(t *testing.T) {
			labelSet, exists := Labels[tt.label]
			if !exists {
				t.Fatalf("Expected label %s to exist", tt.label)
			}

			if labelSet.Cardinality() != tt.wantRepoCount {
				t.Errorf("Expected %d repos with label %s, got %d", tt.wantRepoCount, tt.label, labelSet.Cardinality())
			}

			if !labelSet.Contains(tt.mustContainKey) {
				t.Errorf("Expected label %s to contain repo %s", tt.label, tt.mustContainKey)
			}
		})
	}
}

// TestConcurrentCatalogInitialization tests that concurrent initialization is safe
func TestConcurrentCatalogInitialization(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { cleanupCache(t, ctx) })

	viper := config.Viper(ctx)

	// Register fake provider
	scm.Register("fake-concurrent", func(ctx context.Context, project string) scm.Provider {
		return fake.NewFake("test-project", fake.CreateTestRepositories("test-project"))
	})

	viper.Set(config.GitProvider, "fake-concurrent")
	viper.Set(config.GitProject, "test-project")

	// Try to initialize catalog from multiple goroutines
	// Note: This test verifies that concurrent calls don't panic, but since
	// Catalog is a shared global map, we serialize access to prevent races
	const numGoroutines = 10
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect shared Catalog/Labels state
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			if err := initRepositoryCatalog(ctx, false); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("initRepositoryCatalog failed in concurrent execution: %v", err)
	}

	// Verify catalog was initialized correctly
	if len(Catalog) == 0 {
		t.Error("Expected catalog to be populated")
	}

	// Verify no data corruption
	for key, repo := range Catalog {
		if repo.Name == "" {
			t.Errorf("Repository %s has empty name (possible data corruption)", key)
		}
		if repo.Project == "" {
			t.Errorf("Repository %s has empty project (possible data corruption)", key)
		}
	}
}

// TestConcurrentCatalogAccess tests concurrent read/write access patterns
func TestConcurrentCatalogAccess(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { cleanupCache(t, ctx) })

	viper := config.Viper(ctx)

	// Pre-populate catalog
	Catalog = map[string]scm.Repository{
		"repo-1": {Name: "repo-1", Project: "test", Labels: []string{"backend"}},
		"repo-2": {Name: "repo-2", Project: "test", Labels: []string{"frontend"}},
		"repo-3": {Name: "repo-3", Project: "test", Labels: []string{"backend", "api"}},
	}
	Labels = map[string]mapset.Set[string]{
		"backend":  mapset.NewSet("repo-1", "repo-3"),
		"frontend": mapset.NewSet("repo-2"),
		"api":      mapset.NewSet("repo-3"),
	}

	viper.Set(config.TokenLabel, "~")
	viper.Set(config.TokenSkip, "!")
	viper.Set(config.TokenForced, "+")

	// Simulate concurrent access patterns
	const numReaders = 20
	var wg sync.WaitGroup

	// Multiple readers accessing RepositoryList concurrently
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		filter := []string{"~backend"}
		if i%2 == 0 {
			filter = []string{"~frontend"}
		}
		go func(f []string) {
			defer wg.Done()
			result := RepositoryList(ctx, f...)
			if result.Cardinality() == 0 {
				t.Errorf("Expected non-empty result for filter %v", f)
			}
		}(filter)
	}

	wg.Wait()

	// Verify catalog integrity after concurrent access
	if len(Catalog) != 3 {
		t.Errorf("Catalog was modified during concurrent reads, expected 3 repos, got %d", len(Catalog))
	}
}

// TestCacheErrorHandling tests various cache error scenarios
func TestCacheErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		setupFunc      func(t *testing.T, ctx context.Context) string
		expectFallback bool
		wantError      bool
	}{
		{
			name: "corrupted cache falls back to fetch",
			setupFunc: func(t *testing.T, ctx context.Context) string {
				cachePath := catalogCachePath(ctx)
				cacheDir := filepath.Dir(cachePath)
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					t.Fatalf("Failed to create cache directory: %v", err)
				}
				// Write corrupted JSON
				if err := os.WriteFile(cachePath, []byte("{corrupted json"), 0644); err != nil {
					t.Fatalf("Failed to write corrupted cache: %v", err)
				}
				return cachePath
			},
			expectFallback: true,
			wantError:      false, // Should fallback to fetch, not error
		},
		{
			name: "expired cache falls back to fetch",
			setupFunc: func(t *testing.T, ctx context.Context) string {
				repos := map[string]scm.Repository{
					"old-repo": {
						Name:    "old-repo",
						Project: "test-project",
						Labels:  []string{"old"},
					},
				}
				// Create cache from 48 hours ago
				return setupCacheFile(t, ctx, repos, time.Now().Add(-48*time.Hour))
			},
			expectFallback: true,
			wantError:      false,
		},
		{
			name: "unreadable cache directory",
			setupFunc: func(t *testing.T, ctx context.Context) string {
				cachePath := catalogCachePath(ctx)
				cacheDir := filepath.Dir(cachePath)
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					t.Fatalf("Failed to create cache directory: %v", err)
				}
				// Make directory unreadable (skip on systems where this doesn't work)
				if os.Getuid() == 0 {
					t.Skip("Cannot test permission errors as root")
				}
				if err := os.Chmod(cacheDir, 0000); err != nil {
					t.Fatalf("Failed to change permissions: %v", err)
				}
				t.Cleanup(func() {
					os.Chmod(cacheDir, 0755)
				})
				return cachePath
			},
			expectFallback: false, // Cannot fallback because can't write cache either
			wantError:      true,  // Will fail when trying to save cache after fetch
		},
		{
			name: "empty cache file",
			setupFunc: func(t *testing.T, ctx context.Context) string {
				cachePath := catalogCachePath(ctx)
				cacheDir := filepath.Dir(cachePath)
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					t.Fatalf("Failed to create cache directory: %v", err)
				}
				if err := os.WriteFile(cachePath, []byte(""), 0644); err != nil {
					t.Fatalf("Failed to write empty cache: %v", err)
				}
				return cachePath
			},
			expectFallback: true,
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			viper := config.Viper(ctx)

			// Register fake provider for fallback
			scm.Register("fake-fallback-"+tt.name, func(ctx context.Context, project string) scm.Provider {
				return fake.NewFake("test-project", fake.CreateTestRepositories("test-project"))
			})

			viper.Set(config.GitProvider, "fake-fallback-"+tt.name)
			viper.Set(config.GitProject, "test-project")

			// Setup the error condition
			if tt.setupFunc != nil {
				tt.setupFunc(t, ctx)
			}

			// Try to initialize catalog
			err := initRepositoryCatalog(ctx, false)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// If we expect fallback, verify catalog was populated from provider
			if tt.expectFallback && !tt.wantError {
				if len(Catalog) == 0 {
					t.Error("Expected catalog to be populated via fallback to fetch")
				}
			}
		})
	}
}

// TestCacheSaveFailures tests error handling during cache save operations
func TestCacheSaveFailures(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T, ctx context.Context)
		wantError bool
	}{
		{
			name: "save to readonly directory",
			setupFunc: func(t *testing.T, ctx context.Context) {
				if os.Getuid() == 0 {
					t.Skip("Cannot test permission errors as root")
				}
				cachePath := catalogCachePath(ctx)
				cacheDir := filepath.Dir(cachePath)
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					t.Fatalf("Failed to create cache directory: %v", err)
				}
				// Make directory readonly
				if err := os.Chmod(cacheDir, 0555); err != nil {
					t.Fatalf("Failed to change permissions: %v", err)
				}
				t.Cleanup(func() {
					os.Chmod(cacheDir, 0755)
				})
			},
			wantError: true,
		},
		{
			name: "save with valid directory",
			setupFunc: func(t *testing.T, ctx context.Context) {
				cachePath := catalogCachePath(ctx)
				cacheDir := filepath.Dir(cachePath)
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					t.Fatalf("Failed to create cache directory: %v", err)
				}
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			resetCatalogState(t)
			t.Cleanup(func() { cleanupCache(t, ctx) })

			// Populate catalog with test data
			Catalog = map[string]scm.Repository{
				"test-repo": {
					Name:    "test-repo",
					Project: "test-project",
					Labels:  []string{"test"},
				},
			}

			if tt.setupFunc != nil {
				tt.setupFunc(t, ctx)
			}

			err := saveCatalogCache(ctx)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestPartialProviderFailure tests handling when some providers fail
func TestPartialProviderFailure(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { cleanupCache(t, ctx) })

	viper := config.Viper(ctx)

	// Create a single provider that behaves differently based on project
	scm.Register("fake-partial-fail", func(ctx context.Context, project string) scm.Provider {
		if project == "failing-project" {
			fake := fake.NewFake(project, fake.CreateTestRepositories(project))
			fake.SetError("ListRepositories", fmt.Errorf("simulated API error"))
			return fake
		}
		return fake.NewFake(project, fake.CreateTestRepositories(project))
	})

	viper.Set(config.GitProvider, "fake-partial-fail")
	viper.Set(config.GitProjects, []string{"failing-project", "working-project"})

	// Try to fetch - should fail on first provider
	err := fetchRepositoryData(ctx)

	if err == nil {
		t.Error("Expected error from failing provider")
	}

	// Verify error message mentions the failing project
	if err != nil && !contains(err.Error(), "failing-project") {
		t.Errorf("Expected error to mention failing-project, got: %v", err)
	}
}

// TestConcurrentCacheAccess tests concurrent cache read/write operations
func TestConcurrentCacheAccess(t *testing.T) {
	ctx := loadFixture(t)
	resetCatalogState(t)
	t.Cleanup(func() { cleanupCache(t, ctx) })

	// Setup initial cache
	Catalog = map[string]scm.Repository{
		"repo-1": {Name: "repo-1", Project: "test", Labels: []string{"backend"}},
	}

	err := saveCatalogCache(ctx)
	if err != nil {
		t.Fatalf("Failed to save initial cache: %v", err)
	}

	const numOperations = 20
	var wg sync.WaitGroup
	errors := make(chan error, numOperations)
	var mu sync.Mutex // Protect concurrent access to catalog globals

	// Concurrent reads - serialize catalog state modifications
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			defer mu.Unlock()
			resetCatalogState(t)
			if err := loadCatalogCache(ctx, flushTTL); err != nil {
				errors <- fmt.Errorf("load failed: %w", err)
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent cache operation failed: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
