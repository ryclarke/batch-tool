package git

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

func TestAddDiffCmd(t *testing.T) {
	cmd := addDiffCmd()

	if cmd == nil {
		t.Fatal("addDiffCmd() returned nil")
	}

	if cmd.Use != "diff <repository>..." {
		t.Errorf("Expected Use to be 'diff <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestDiffCmdArgs(t *testing.T) {
	cmd := addDiffCmd()

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

func TestDiffCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})

	// Modify a file to create a diff
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("modified content\n"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	testCtx := setupTestGitContext(t, reposPath)

	cmd := addDiffCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("repo-1")) {
		t.Errorf("Expected output to contain 'repo-1', got: %s", output)
	}
}

func TestDiffCommandRunMultipleRepos(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"})

	// Modify files in both repos to create diffs
	for _, repo := range []string{"repo-1", "repo-2"} {
		repoDir := filepath.Join(reposPath, "example.com", "test-project", repo)
		testFile := filepath.Join(repoDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("modified content in "+repo+"\n"), 0644); err != nil {
			t.Fatalf("Failed to modify test file in %s: %v", repo, err)
		}
	}

	testCtx := setupTestGitContext(t, reposPath)

	cmd := addDiffCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1", "repo-2"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	for _, repo := range []string{"repo-1", "repo-2"} {
		if !bytes.Contains([]byte(output), []byte(repo)) {
			t.Errorf("Expected output to contain '%s', got: %s", repo, output)
		}
	}
}
