package utils

import (
	"path/filepath"
	"strings"
	"testing"
)

// checkError verifies error expectation
func checkError(t *testing.T, err error, wantError bool) {
	t.Helper()
	if (err != nil) != wantError {
		t.Errorf("error = %v, wantError %v", err, wantError)
	}
}

// checkStringEqual verifies two strings are equal
func checkStringEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// checkStringContains verifies a string contains expected substring
func checkStringContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("Expected string to contain %q, got %q", want, got)
	}
}

// checkStringNotEmpty verifies a string is not empty
func checkStringNotEmpty(t *testing.T, got string) {
	t.Helper()
	if got == "" {
		t.Error("Expected non-empty string")
	}
}

// checkAbsolutePath verifies a path is absolute
func checkAbsolutePath(t *testing.T, path string) {
	t.Helper()
	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %q", path)
	}
}
