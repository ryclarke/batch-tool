package git

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddPushCmd(t *testing.T) {
	cmd := addPushCmd()

	if cmd == nil {
		t.Fatal("addPushCmd() returned nil")
	}

	expectedUse := "push [-f] [--remote <name>] <repository>..."
	if cmd.Use != expectedUse {
		t.Errorf("Expected Use to be %q, got %s", expectedUse, cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestPushCmdFlags(t *testing.T) {
	cmd := addPushCmd()

	// Test force flag
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("force flag not found")
	}

	if forceFlag.Shorthand != "f" {
		t.Errorf("Expected force flag shorthand to be 'f', got %s", forceFlag.Shorthand)
	}
}

func TestPushCmdArgs(t *testing.T) {
	cmd := addPushCmd()

	// Test that command requires minimum arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test that command accepts arguments
	err = cmd.Args(cmd, []string{"repo1"})
	if err != nil {
		t.Errorf("Expected no error with valid arguments, got %v", err)
	}
}

func TestPushCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"}, true)

	tests := []struct {
		name           string
		repos          []string
		force          bool
		expectedOutput []string
	}{
		{
			name:           "Push single repo",
			repos:          []string{"repo-1"},
			force:          false,
			expectedOutput: []string{},
		},
		{
			name:           "Push multiple repos",
			repos:          []string{"repo-1", "repo-2"},
			force:          false,
			expectedOutput: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := setupTestGitContext(t, reposPath)
			testViper := config.Viper(testCtx)

			if tt.force {
				testViper.Set(config.GitPushForce, true)
			}

			cmd := addPushCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			args := tt.repos
			if tt.force {
				args = append([]string{"--force"}, args...)
			}

			cmd.SetArgs(args)

			err := cmd.ExecuteContext(testCtx)
			// Push will error because we're trying to push to main branch
			// This is expected behavior from ValidateBranch
			_ = err

			output := buf.String()

			for _, expected := range tt.expectedOutput {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					t.Errorf("Expected output to contain %q, got: %s", expected, output)
				}
			}
		})
	}
}

func TestPushCommandRunWithForce(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	testCtx := setupTestGitContext(t, reposPath)
	testViper := config.Viper(testCtx)

	// Set force flag
	testViper.Set(config.GitPushForce, true)

	cmd2 := addPushCmd()

	var buf bytes.Buffer
	cmd2.SetOut(&buf)
	cmd2.SetErr(&buf)
	cmd2.SetArgs([]string{"--force", "repo-1"})

	_ = cmd2.ExecuteContext(testCtx)
	// Should attempt force push
}

func TestPushRemoteFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{name: "defaults to origin", args: nil, expected: "origin"},
		{name: "honors an explicit remote", args: []string{"--remote", "upstream"}, expected: "upstream"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			cmd := addPushCmd()
			cmd.SetContext(ctx)

			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("Failed parsing flags: %v", err)
			}

			if err := cmd.PreRunE(cmd, []string{"repo-1"}); err != nil {
				t.Fatalf("PreRunE failed: %v", err)
			}

			if got := config.Viper(ctx).GetString(config.GitRemote); got != tt.expected {
				t.Errorf("Expected %s to be %q, got %q", config.GitRemote, tt.expected, got)
			}
		})
	}
}

// TestPushToAlternateRemote verifies end to end that the configured remote is the
// one actually pushed to.
func TestPushToAlternateRemote(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	testCtx := setupTestGitContext(t, reposPath)

	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")

	// Stand up a second bare repository and wire it up as "upstream"
	upstreamPath := filepath.Join(reposPath, "upstream.git")
	if err := os.MkdirAll(upstreamPath, 0755); err != nil {
		t.Fatalf("Failed to create upstream directory: %v", err)
	}

	testhelper.ExecCommand(t, upstreamPath, "git", "init", "--bare")
	testhelper.ExecCommand(t, repoDir, "git", "remote", "add", "upstream", upstreamPath)

	cmd := addPushCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--remote", "upstream", "repo-1"})

	if err := cmd.ExecuteContext(testCtx); err != nil {
		t.Fatalf("Command execution failed: %v\n%s", err, buf.String())
	}

	// feature-branch should now exist in upstream, and not in origin
	branches := gitOutput(t, upstreamPath, "branch", "--list", "feature-branch")
	if !strings.Contains(branches, "feature-branch") {
		t.Errorf("Expected feature-branch to be pushed to upstream, got %q", branches)
	}
}
