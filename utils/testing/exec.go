package testing

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ExecCommand creates and runs an exec.Command with the given working directory.
// Common utility for all test packages needing to run git or shell commands.
func ExecCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if err != nil {
		var outStr string
		if len(output) > 0 {
			outStr = "\n" + string(output)
		}

		t.Fatalf("Command %s %v failed: %v%s", name, args, err, outStr)
	}
}

// SetupRepos creates git repositories that mimic real git workflow.
// Creates a shared bare origin repository and clones from it.
// Tests are responsible for creating their own branches as needed.
func SetupRepos(t *testing.T, repos []string, branches ...bool) string {
	t.Helper()

	tmpDir := t.TempDir()
	host := "example.com"
	project := "test-project"

	// Create shared bare origin repository
	originPath := filepath.Join(tmpDir, "origin.git")
	if err := os.MkdirAll(originPath, 0755); err != nil {
		t.Fatalf("Failed to create origin directory: %v", err)
	}

	ExecCommand(t, originPath, "git", "init", "--bare")

	// Process first repository
	if len(repos) > 0 {
		firstRepoDir := filepath.Join(tmpDir, host, project, repos[0])
		if err := os.MkdirAll(firstRepoDir, 0755); err != nil {
			t.Fatalf("Failed to create test repo directory: %v", err)
		}

		initCmds := [][]string{
			{"git", "init"},
			{"git", "config", "user.email", "test@example.com"},
			{"git", "config", "user.name", "Test User"},
			{"git", "checkout", "-b", "main"},
			{"git", "commit", "--allow-empty", "-m", "Initial commit"},
			{"git", "remote", "add", "origin", originPath},
			{"git", "push", "-u", "origin", "main"},
		}

		for _, cmdArgs := range initCmds {
			ExecCommand(t, firstRepoDir, cmdArgs[0], cmdArgs[1:]...)
		}

		// Create test file
		testFile := filepath.Join(firstRepoDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		if len(branches) > 0 && branches[0] {
			ExecCommand(t, firstRepoDir, "git", "checkout", "-b", "feature-branch")
		}
	}

	// Process additional repositories
	for i := 1; i < len(repos); i++ {
		repoDir := filepath.Join(tmpDir, host, project, repos[i])

		// Clone from origin
		parentDir := filepath.Dir(repoDir)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			t.Fatalf("Failed to create parent directory: %v", err)
		}

		ExecCommand(t, parentDir, "git", "clone", originPath, repos[i])

		// Configure git user
		for _, cmdArgs := range [][]string{
			{"git", "config", "user.email", "test@example.com"},
			{"git", "config", "user.name", "Test User"},
		} {
			ExecCommand(t, repoDir, cmdArgs[0], cmdArgs[1:]...)
		}

		// Create test file
		testFile := filepath.Join(repoDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		if len(branches) > 0 && branches[0] {
			ExecCommand(t, repoDir, "git", "checkout", "-b", "feature-branch")
		}
	}

	return tmpDir
}
