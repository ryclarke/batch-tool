package output_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	"github.com/spf13/cobra"
)

func loadFixture(t *testing.T) context.Context {
	t.Helper()
	return config.LoadFixture(t, "../../config")
}

// checkOutputContains checks that all expected messages are present in the output (supports map or slice)
func checkOutputContains(t *testing.T, got string, want any) {
	t.Helper()

	switch wantIter := want.(type) {
	case map[string]string:
		for key, msg := range wantIter {
			if !strings.Contains(got, msg) {
				t.Errorf("Expected output for %s to contain %s", key, msg)
			}
		}
	case []string:
		for _, msg := range wantIter {
			if !strings.Contains(got, msg) {
				t.Errorf("Expected output to contain %s", msg)
			}
		}
	}
}

// setupDirs creates the repository directories so that Do won't try to clone them
func setupDirs(t *testing.T, ctx context.Context, repos []string) {
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

// fakeCmd creates a test cobra command with the given context and output writer
func fakeCmd(t *testing.T, ctx context.Context, out io.Writer) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.SetOut(out)

	return cmd
}
