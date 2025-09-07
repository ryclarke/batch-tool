package multichange

import (
	"os"
	"path/filepath"
)

// getChangesDir returns the standard path for the changes directory
// It creates the directory if it doesn't exist
func getChangesDir() (string, error) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		// Fall back to default GOPATH if not set
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		gopath = filepath.Join(homeDir, "go")
	}

	changesDir := filepath.Join(gopath, "src", "changes")

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(changesDir, 0755); err != nil {
		return "", err
	}

	return changesDir, nil
}
