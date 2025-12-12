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
	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

func loadFixture(t *testing.T) context.Context {
	t.Helper()
	return testhelper.LoadFixture(t, "../config")
}

// setupCacheFile creates a cache file with the given repositories and timestamp
func setupCacheFile(t *testing.T, ctx context.Context, repos map[string]scm.Repository, updatedAt time.Time) string {
	t.Helper()

	cachePath := catalogCachePath(ctx)

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
