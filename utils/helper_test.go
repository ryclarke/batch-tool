package utils_test

import (
	"context"
	"path/filepath"
	"testing"

	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

// loadFixture loads test configuration fixture
func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../config")
}

// checkAbsolutePath verifies a path is absolute
func checkAbsolutePath(t *testing.T, path string) {
	t.Helper()

	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %q", path)
	}
}
