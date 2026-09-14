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

func TestAddUpdateCmd(t *testing.T) {
	cmd := addUpdateCmd()

	if cmd == nil {
		t.Fatal("addUpdateCmd() returned nil")
	}

	expectedUse := "update [--stash] [--pull-strategy <strategy>] <repository>..."
	if cmd.Use != expectedUse {
		t.Errorf("Expected Use to be %q, got %s", expectedUse, cmd.Use)
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

	// Set update stash to true in config
	viper := config.Viper(testCtx)
	viper.Set(config.GitUpdateStash, true)

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

	// Set update stash to true in config
	viper := config.Viper(testCtx)
	viper.Set(config.GitUpdateStash, true)

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

func TestUpdatePullStrategyInvalid(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	cmd := addUpdateCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--pull-strategy", "yolo", "repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err == nil {
		t.Fatal("Expected error for an invalid --pull-strategy value")
	}

	if !strings.Contains(err.Error(), config.GitUpdatePullStrategy) {
		t.Errorf("Expected error to reference %s, got: %v", config.GitUpdatePullStrategy, err)
	}
}

func TestUpdatePullStrategyBinding(t *testing.T) {
	for _, strategy := range AvailablePullStrategies {
		t.Run(strategy, func(t *testing.T) {
			ctx := loadFixture(t)
			cmd := addUpdateCmd()
			cmd.SetContext(ctx)

			if err := cmd.ParseFlags([]string{"--pull-strategy", strategy}); err != nil {
				t.Fatalf("Failed parsing flags: %v", err)
			}

			if err := cmd.PreRunE(cmd, []string{"repo-1"}); err != nil {
				t.Fatalf("PreRunE failed: %v", err)
			}

			if got := config.Viper(ctx).GetString(config.GitUpdatePullStrategy); got != strategy {
				t.Errorf("Expected %s to be %q, got %q", config.GitUpdatePullStrategy, strategy, got)
			}
		})
	}
}

// TestCleanWithoutSubmodules verifies that disabling git.update.submodules skips the
// submodule steps while still discarding local changes.
func TestCleanWithoutSubmodules(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)
	config.Viper(testCtx).Set(config.GitUpdateSubmodules, false)

	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")

	untracked := filepath.Join(repoDir, "untracked.txt")
	if err := os.WriteFile(untracked, []byte("untracked\n"), 0644); err != nil {
		t.Fatalf("Failed to create untracked file: %v", err)
	}

	cmd := addUpdateCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--no-stash", "repo-1"})

	if err := cmd.ExecuteContext(testCtx); err != nil {
		t.Fatalf("Command execution failed: %v\n%s", err, buf.String())
	}

	if _, err := os.Stat(untracked); !os.IsNotExist(err) {
		t.Error("Expected untracked file to be cleaned")
	}

	if strings.Contains(buf.String(), "submodule") {
		t.Errorf("Expected no submodule commands to run, got: %s", buf.String())
	}
}

// TestCleanIgnoredFiles verifies that git.update.clean-ignored extends `git clean`
// to ignored files, which the default -fd does not remove.
func TestCleanIgnoredFiles(t *testing.T) {
	for _, cleanIgnored := range []bool{false, true} {
		name := "keeps ignored files"
		if cleanIgnored {
			name = "removes ignored files"
		}

		t.Run(name, func(t *testing.T) {
			reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
			testCtx := setupTestGitContext(t, reposPath)
			config.Viper(testCtx).Set(config.GitUpdateCleanIgnored, cleanIgnored)

			repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")

			if err := os.WriteFile(filepath.Join(repoDir, ".gitignore"), []byte("ignored.txt\n"), 0644); err != nil {
				t.Fatalf("Failed to create gitignore: %v", err)
			}
			testhelper.ExecCommand(t, repoDir, "git", "add", ".gitignore")
			testhelper.ExecCommand(t, repoDir, "git", "commit", "-m", "Add gitignore")

			ignored := filepath.Join(repoDir, "ignored.txt")
			if err := os.WriteFile(ignored, []byte("ignored\n"), 0644); err != nil {
				t.Fatalf("Failed to create ignored file: %v", err)
			}

			cmd := addUpdateCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{"--no-stash", "repo-1"})

			if err := cmd.ExecuteContext(testCtx); err != nil {
				t.Fatalf("Command execution failed: %v\n%s", err, buf.String())
			}

			if _, err := os.Stat(ignored); os.IsNotExist(err) != cleanIgnored {
				t.Errorf("Expected ignored file removed=%v", cleanIgnored)
			}
		})
	}
}
