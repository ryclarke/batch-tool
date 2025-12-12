/* Package test provides utility functions for testing purposes across multiple packages.
 */
package test

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	"github.com/spf13/cobra"
)

// LoadFixture loads test configuration from the config package.
// The configPath parameter should be the relative path from the test file
// to the config directory (e.g., "../config", "../../config").
func LoadFixture(t *testing.T, configPath string) context.Context {
	t.Helper()

	v := config.New()
	ctx := config.SetViper(context.Background(), v)

	v.SetConfigName("fixture")
	v.AddConfigPath(configPath)

	if err := v.ReadInConfig(); err != nil {
		t.Fatalf("Failed to load fixture config: %v", err)
	}

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
