// Package testing provides utility functions for testing purposes across multiple packages.
package testing

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

// LoadFixture loads test configuration from the config package.
// The configPath parameter should be the relative path from the test file
// to the config directory (e.g., "../config", "../../config").
func LoadFixture(t *testing.T, configPath string) context.Context {
	t.Helper()

	viper := config.New()
	ctx := config.SetViper(context.Background(), viper)

	viper.SetConfigName("fixture")
	viper.AddConfigPath(configPath)

	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to load fixture config: %v", err)
	}

	// Override GitDirectory with a temporary directory for test isolation and reliable cleanup
	tempDir := t.TempDir()
	viper.Set(config.GitDirectory, tempDir)

	return ctx
}

// SetupDirs creates repository directories to prevent cloning during tests.
// Automatically registers cleanup to remove the directories after the test.
func SetupDirs(t *testing.T, ctx context.Context, repos []string) {
	t.Helper()

	for _, repo := range repos {
		repoPath := utils.RepoPath(ctx, repo)
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			t.Fatalf("Failed to create repo directory %s: %v", repoPath, err)
		}
	}

	// Clean up after test
	t.Cleanup(func() {
		gitDir := config.Viper(ctx).GetString(config.GitDirectory)
		os.RemoveAll(gitDir)
	})
}

// FakeCmd creates a minimal cobra.Command for testing with the given context and output writer.
func FakeCmd(t *testing.T, ctx context.Context, out io.Writer) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.SetContext(ctx)
	cmd.SetOut(out)
	cmd.SetErr(out)

	return cmd
}
