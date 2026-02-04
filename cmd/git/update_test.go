package git

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddUpdateCmd(t *testing.T) {
	cmd := addUpdateCmd()

	if cmd == nil {
		t.Fatal("addUpdateCmd() returned nil")
	}

	if cmd.Use != "update <repository>..." {
		t.Errorf("Expected Use to be 'update <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestUpdateCmdArgs(t *testing.T) {
	cmd := addUpdateCmd()

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

func TestUpdateCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"})

	tests := []struct {
		name           string
		repos          []string
		expectedOutput []string
	}{
		{
			name:  "Update single repo",
			repos: []string{"repo-1"},
			expectedOutput: []string{
				"repo-1",
			},
		},
		{
			name:  "Update multiple repos",
			repos: []string{"repo-1", "repo-2"},
			expectedOutput: []string{
				"repo-1",
				"repo-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := setupTestGitContext(t, reposPath)

			cmd := addUpdateCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.repos)

			err := cmd.ExecuteContext(testCtx)
			if err != nil {
				t.Fatalf("Command execution failed: %v", err)
			}

			output := buf.String()

			for _, expected := range tt.expectedOutput {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					t.Errorf("Expected output to contain %q, got: %s", expected, output)
				}
			}
		})
	}
}

func TestUpdateCommandRunFromFeatureBranch(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Setup repository state - create feature branch
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testhelper.ExecCommand(t, repoDir, "git", "checkout", "-b", "feature-branch")

	cmdUpdate := addUpdateCmd()

	var buf bytes.Buffer
	cmdUpdate.SetOut(&buf)
	cmdUpdate.SetErr(&buf)
	cmdUpdate.SetArgs([]string{"repo-1"})

	err := cmdUpdate.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("repo-1")) {
		t.Errorf("Expected output to contain 'repo-1', got: %s", output)
	}
}

func TestCleanFunction(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Create uncommitted changes and untracked files
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "uncommitted.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted change"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	untrackedFile := filepath.Join(repoDir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("untracked file"), 0644); err != nil {
		t.Fatalf("Failed to create untracked file: %v", err)
	}

	// Stage one file
	testhelper.ExecCommand(t, repoDir, "git", "add", "uncommitted.txt")

	cmd := addUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// Verify both files are removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected staged file to be cleaned")
	}

	if _, err := os.Stat(untrackedFile); !os.IsNotExist(err) {
		t.Error("Expected untracked file to be cleaned")
	}
}

func TestUpdateWithStashFlag(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Create uncommitted changes
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "stashed-change.txt")
	testContent := []byte("content to be stashed")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	testhelper.ExecCommand(t, repoDir, "git", "add", "stashed-change.txt")

	cmd := addUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--stash", "repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// Verify file is restored after update
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Expected file to be restored after stash pop")
	}

	// Verify content is preserved
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}
	if !bytes.Equal(content, testContent) {
		t.Errorf("Expected restored content to match original, got %s", string(content))
	}
}

func TestUpdateWithNoStashFlag(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Set stash-updates to true in config
	viper := config.Viper(testCtx)
	viper.Set(config.StashUpdates, true)

	// Create uncommitted changes
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "to-be-cleaned.txt")
	if err := os.WriteFile(testFile, []byte("will be destroyed"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	testhelper.ExecCommand(t, repoDir, "git", "add", "to-be-cleaned.txt")

	cmd := addUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--no-stash", "repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// Verify file is cleaned (not stashed)
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected file to be cleaned, not stashed")
	}
}

func TestUpdateWithConfigStash(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Set stash-updates to true in config
	viper := config.Viper(testCtx)
	viper.Set(config.StashUpdates, true)

	// Create uncommitted changes
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "config-stashed.txt")
	testContent := []byte("stashed by config")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	testhelper.ExecCommand(t, repoDir, "git", "add", "config-stashed.txt")

	cmd := addUpdateCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// Verify file is restored (stashed and popped due to config)
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("Expected file to be restored after stash pop")
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}
	if !bytes.Equal(content, testContent) {
		t.Errorf("Expected restored content to match original")
	}
}
