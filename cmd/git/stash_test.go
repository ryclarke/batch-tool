package git

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddStashCmd(t *testing.T) {
	cmd := addStashCmd()

	if cmd == nil {
		t.Fatal("addStashCmd() returned nil")
	}

	if cmd.Use != "stash <repository>..." {
		t.Errorf("Expected Use to be 'stash <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("Expected Long description to be set")
	}

	// Verify subcommands exist
	pushCmd := cmd.Commands()
	if len(pushCmd) != 2 {
		t.Errorf("Expected 2 subcommands (push and pop), got %d", len(pushCmd))
	}
}

func TestStashCmdArgs(t *testing.T) {
	cmd := addStashCmd()

	// The stash command itself doesn't require arguments (it lists stashes by default)
	// But its subcommands do require repository arguments

	// Test push subcommand
	pushCmd := findSubcommand(cmd, "push")
	if pushCmd == nil {
		t.Fatal("push subcommand not found")
	}

	if pushCmd.Args == nil {
		t.Error("Expected push subcommand to have Args validator")
	}

	// Test pop subcommand
	popCmd := findSubcommand(cmd, "pop")
	if popCmd == nil {
		t.Fatal("pop subcommand not found")
	}

	if popCmd.Args == nil {
		t.Error("Expected pop subcommand to have Args validator")
	}
}

// Helper function to find subcommand by name
func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}

func TestStashPushCleanWorktree(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	cmd := addStashCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"push", "repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()
	// With a clean worktree, stash push should report "Nothing to stash" or succeed silently
	if !bytes.Contains([]byte(output), []byte("Nothing to stash")) &&
		!bytes.Contains([]byte(output), []byte("worktree is clean")) &&
		!bytes.Contains([]byte(output), []byte("No local changes")) &&
		err != nil {
		t.Errorf("Expected clean worktree message or success, got: %s, err: %v", output, err)
	}
}

func TestStashPushWithChanges(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Create uncommitted changes
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "test-change.txt")
	if err := os.WriteFile(testFile, []byte("uncommitted change"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	// Track the file with git so it shows as a change
	testhelper.ExecCommand(t, repoDir, "git", "add", "test-change.txt")

	cmd := addStashCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"push", "repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// Verify the file is no longer present (stashed)
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected test file to be stashed (removed from worktree)")
	}
}

func TestStashPopEmptyStash(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	cmd := addStashCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"pop", "repo-1"})

	// Pop without prior stash should succeed (no-op)
	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Expected no error when popping without stashed state, got: %v", err)
	}
}

func TestStashPushPopRoundTrip(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Create uncommitted changes
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "roundtrip-test.txt")
	testContent := []byte("roundtrip content")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	testhelper.ExecCommand(t, repoDir, "git", "add", "roundtrip-test.txt")

	// Push to stash using shared context
	pushCmd := addStashCmd()
	var pushBuf bytes.Buffer
	pushCmd.SetOut(&pushBuf)
	pushCmd.SetErr(&pushBuf)
	pushCmd.SetContext(testCtx)
	pushCmd.SetArgs([]string{"push", "repo-1"})

	if err := pushCmd.ExecuteContext(testCtx); err != nil {
		t.Fatalf("Stash push failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Expected test file to be stashed")
	}

	// Pop from stash using same context
	popCmd := addStashCmd()
	var popBuf bytes.Buffer
	popCmd.SetOut(&popBuf)
	popCmd.SetErr(&popBuf)
	popCmd.SetContext(testCtx)
	popCmd.SetArgs([]string{"pop", "repo-1"})

	if err := popCmd.ExecuteContext(testCtx); err != nil {
		t.Fatalf("Stash pop failed: %v", err)
	}

	// Verify file is restored
	restored, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}
	if !bytes.Equal(restored, testContent) {
		t.Errorf("Restored content mismatch: expected %q, got %q", testContent, restored)
	}
}

func TestStashInvalidAction(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	cmd := addStashCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"invalid", "repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err == nil {
		t.Fatal("Expected error for invalid action")
	}

	// With subcommands, cobra returns "unknown command" error
	if !bytes.Contains([]byte(err.Error()), []byte("unknown command")) {
		t.Errorf("Expected 'unknown command' error, got: %v", err)
	}
}

func TestLookupChanges(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// SetupRepos creates test.txt but doesn't commit it - commit it so repo is clean
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testhelper.ExecCommand(t, repoDir, "git", "add", "test.txt")
	testhelper.ExecCommand(t, repoDir, "git", "commit", "-m", "Add test.txt")

	// Now repo should be clean
	hasChanges, err := lookupChanges(testCtx, "repo-1")
	if err != nil {
		t.Fatalf("lookupChanges failed: %v", err)
	}
	if hasChanges {
		t.Error("Expected clean worktree to have no uncommitted changes")
	}

	// Create a new file (untracked)
	testFile := filepath.Join(repoDir, "uncommitted.txt")
	if err := os.WriteFile(testFile, []byte("change"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Should now detect changes (untracked file shows in --porcelain)
	hasChanges, err = lookupChanges(testCtx, "repo-1")
	if err != nil {
		t.Fatalf("lookupChanges failed: %v", err)
	}
	if !hasChanges {
		t.Error("Expected worktree with new file to have uncommitted changes")
	}
}

func TestLookupStash(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Clean repo should have no stash entries
	message, err := lookupStash(testCtx, "repo-1")
	if err != nil {
		t.Fatalf("lookupStash failed: %v", err)
	}
	if message != "" {
		t.Errorf("Expected empty message for fresh repo, got: %s", message)
	}

	// Create a file change and stash it
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "stash-test.txt")
	if err := os.WriteFile(testFile, []byte("stash me"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	testhelper.ExecCommand(t, repoDir, "git", "add", "stash-test.txt")
	// Actually stash it
	testhelper.ExecCommand(t, repoDir, "git", "stash", "push", "-m", "test stash")

	// Should now return a non-empty stash message
	message, err = lookupStash(testCtx, "repo-1")
	if err != nil {
		t.Fatalf("lookupStash failed: %v", err)
	}
	if message == "" {
		t.Error("Expected non-empty stash message after stashing")
	}
}

func TestValidateStash(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	testCtx := setupTestGitContext(t, reposPath)

	// Create a mock channel for testing
	ch := testhelper.NewMockChannel("repo-1")

	// Should fail with no stash
	err := ValidateStash(testCtx, ch)
	if err == nil {
		t.Error("Expected error when validating empty stash")
	}

	// Create a file change and stash it with batch-tool prefix
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testFile := filepath.Join(repoDir, "validate-test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	// Stash needs actual change - git won't stash just untracked files with 'push'
	testhelper.ExecCommand(t, repoDir, "git", "add", "validate-test.txt")
	testhelper.ExecCommand(t, repoDir, "git", "stash", "push", "-m", "batch-tool 2026-01-08T12:00:00Z")

	// Should succeed with batch-tool stash (message may have branch prefix)
	err = ValidateStash(testCtx, ch)
	if err != nil {
		t.Errorf("Expected no error with batch-tool stash, got: %v", err)
	}
}

func TestStashMultipleRepos(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"})
	testCtx := setupTestGitContext(t, reposPath)

	// Commit the test.txt file that SetupRepos created to ensure repo has a commit history
	for _, repoName := range []string{"repo-1", "repo-2"} {
		repoDir := filepath.Join(reposPath, "example.com", "test-project", repoName)
		testhelper.ExecCommand(t, repoDir, "git", "add", "test.txt")
		testhelper.ExecCommand(t, repoDir, "git", "commit", "-m", "Add test.txt")
	}

	// Create changes in both repos
	for _, repoName := range []string{"repo-1", "repo-2"} {
		repoDir := filepath.Join(reposPath, "example.com", "test-project", repoName)
		testFile := filepath.Join(repoDir, "multi-test.txt")
		if err := os.WriteFile(testFile, []byte("content-"+repoName), 0644); err != nil {
			t.Fatalf("Failed to create test file in %s: %v", repoName, err)
		}
		testhelper.ExecCommand(t, repoDir, "git", "add", "multi-test.txt")
	}

	cmd := addStashCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"push", "repo-1", "repo-2"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// Verify both repos had files stashed
	for _, repoName := range []string{"repo-1", "repo-2"} {
		repoDir := filepath.Join(reposPath, "example.com", "test-project", repoName)
		testFile := filepath.Join(repoDir, "multi-test.txt")
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Errorf("Expected test file in %s to be stashed", repoName)
		}
	}
}
