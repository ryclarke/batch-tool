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
)

func loadFixture(t *testing.T) context.Context {
	t.Helper()
	return config.LoadFixture(t, "../config")
}

// checkError validates test error against expected results
func checkError(t *testing.T, err error, wantErr bool) bool {
	t.Helper()

	if wantErr != (err != nil) {
		t.Errorf("Expected error = %v, got: %v", wantErr, err)
		return false
	}

	return true
}

// checkCatalogSize validates the catalog has the expected number of repositories
func checkCatalogSize(t *testing.T, wantSize int) {
	t.Helper()

	if len(Catalog) != wantSize {
		t.Errorf("Expected catalog size = %d, got %d", wantSize, len(Catalog))
	}
}

// checkRepoCount validates the number of repositories matches expected count
func checkRepoCount(t *testing.T, got []string, wantCount int) {
	t.Helper()

	if len(got) != wantCount {
		t.Errorf("Expected %d repositories, got %d", wantCount, len(got))
	}
}

// checkRepoContains validates that all expected repos are in the list
func checkRepoContains(t *testing.T, got []string, wantRepos []string) {
	t.Helper()

	repoSet := mapset.NewSet(got...)
	for _, repo := range wantRepos {
		if !repoSet.Contains(repo) {
			t.Errorf("Expected %s in repository list", repo)
		}
	}
}

// checkRepoNotContains validates that unwanted repos are not in the list
func checkRepoNotContains(t *testing.T, got []string, unwantedRepos []string) {
	t.Helper()

	repoSet := mapset.NewSet(got...)
	for _, repo := range unwantedRepos {
		if repoSet.Contains(repo) {
			t.Errorf("Expected %s to NOT be in repository list", repo)
		}
	}
}

// setupCacheFile creates a cache file with the given repositories and timestamp
func setupCacheFile(t *testing.T, ctx context.Context, repos map[string]scm.Repository, updatedAt time.Time) string {
	t.Helper()

	viper := config.Viper(ctx)
	cachePath := filepath.Join(viper.GetString(config.GitDirectory),
		viper.GetString(config.GitHost),
		viper.GetString(config.GitProject),
		viper.GetString(config.CatalogCacheFile),
	)

	cache := repositoryCache{
		UpdatedAt:    updatedAt,
		Repositories: repos,
	}

	data, err := json.Marshal(&cache)
	if err != nil {
		t.Fatalf("Failed to marshal cache: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	return cachePath
}

// cleanupCache removes the cache file and resets the catalog state
func cleanupCache(t *testing.T, ctx context.Context) {
	t.Helper()

	viper := config.Viper(ctx)
	gitDir := viper.GetString(config.GitDirectory)

	// Reset global state FIRST before removing files
	Catalog = make(map[string]scm.Repository)
	Labels = make(map[string]mapset.Set[string])

	// Remove test directory
	if err := os.RemoveAll(gitDir); err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: failed to remove test directory: %v", err)
	}
}

// resetCatalogState resets the catalog and labels to empty state
func resetCatalogState(t *testing.T) {
	t.Helper()

	Catalog = make(map[string]scm.Repository)
	Labels = make(map[string]mapset.Set[string])
}
