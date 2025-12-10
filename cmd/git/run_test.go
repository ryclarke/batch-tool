package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

// setupTestGitRepos creates temporary git repositories for testing
func setupTestGitRepos(t *testing.T, repos []string) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create a bare repository to act as the remote origin
	originPath := filepath.Join(tmpDir, "origin.git")
	if err := os.MkdirAll(originPath, 0755); err != nil {
		t.Fatalf("Failed to create origin directory: %v", err)
	}

	cmd := testGitExecCommand(t, originPath, "git", "init", "--bare")
	var initOut bytes.Buffer
	cmd.Stdout = &initOut
	cmd.Stderr = &initOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init bare repo: %v\nOutput: %s", err, initOut.String())
	}

	// Create directory structure: tmpDir/host/project/repo
	host := "example.com"
	project := "test-project"

	for _, repo := range repos {
		repoDir := filepath.Join(tmpDir, host, project, repo)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatalf("Failed to create test repo directory: %v", err)
		}

		// Initialize a real git repository with remote tracking
		cmds := [][]string{
			{"git", "init"},
			{"git", "config", "user.email", "test@example.com"},
			{"git", "config", "user.name", "Test User"},
			{"git", "checkout", "-b", "main"},
			{"git", "commit", "--allow-empty", "-m", "Initial commit"},
			{"git", "remote", "add", "origin", originPath},
			{"git", "branch", "-M", "main"}, // Ensure main branch is the current branch
			{"git", "push", "-u", "origin", "main"},
		}

		for _, cmdArgs := range cmds {
			cmd := testGitExecCommand(t, repoDir, cmdArgs[0], cmdArgs[1:]...)
			var cmdOut bytes.Buffer
			cmd.Stdout = &cmdOut
			cmd.Stderr = &cmdOut
			if err := cmd.Run(); err != nil {
				t.Fatalf("Failed to run git command %v in %s: %v\nOutput: %s", cmdArgs, repoDir, err, cmdOut.String())
			}
		}

		// Create a test file so we have something to commit
		testFile := filepath.Join(repoDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content\n"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	return tmpDir
}

// testGitExecCommand creates an exec.Command with the given working directory
func testGitExecCommand(t *testing.T, dir string, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd
}

// setupTestGitContext configures a test context for git commands
func setupTestGitContext(t *testing.T, reposPath string) context.Context {
	t.Helper()

	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Configure viper for testing
	viper.Set(config.GitDirectory, reposPath)
	viper.Set(config.GitHost, "example.com")
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.SourceBranch, "main")
	viper.Set(config.MaxConcurrency, 1) // Serial execution for predictable test output
	viper.Set(config.ChannelBuffer, 10)

	return ctx
}

func TestBranchCommandRun(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1", "repo-2"})

	tests := []struct {
		name           string
		branch         string
		repos          []string
		expectedOutput []string
	}{
		{
			name:   "Checkout new branch",
			branch: "feature-branch",
			repos:  []string{"repo-1"},
			expectedOutput: []string{
				"repo-1",
			},
		},
		{
			name:   "Checkout branch multiple repos",
			branch: "another-feature",
			repos:  []string{"repo-1", "repo-2"},
			expectedOutput: []string{
				"repo-1",
				"repo-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := setupTestGitContext(t, reposPath)
			testViper := config.Viper(testCtx)
			testViper.Set(config.Branch, tt.branch)

			cmd := addBranchCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(append([]string{"--branch", tt.branch}, tt.repos...))

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

func TestBranchCommandRunWithoutBranch(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1"})
	ctx := setupTestGitContext(t, reposPath)

	cmd := addBranchCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	err := cmd.ExecuteContext(ctx)
	if err == nil {
		t.Error("Expected error when branch not provided")
	}
}

func TestCommitCommandRun(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1", "repo-2"})

	tests := []struct {
		name           string
		message        string
		amend          bool
		repos          []string
		expectedOutput []string
		setupFunc      func(t *testing.T, repoPath string)
	}{
		{
			name:    "Commit with message",
			message: "Test commit message",
			amend:   false,
			repos:   []string{"repo-1"},
			expectedOutput: []string{
				"repo-1",
			},
			setupFunc: func(t *testing.T, repoPath string) {
				// Create feature branch
				cmd := testGitExecCommand(t, repoPath, "git", "checkout", "-b", "feature-branch")
				if err := cmd.Run(); err != nil {
					t.Fatalf("Failed to create feature branch: %v", err)
				}
			},
		},
		{
			name:    "Amend commit",
			message: "",
			amend:   true,
			repos:   []string{"repo-1"},
			expectedOutput: []string{
				"repo-1",
			},
			setupFunc: func(t *testing.T, repoPath string) {
				// Create feature branch
				cmd := testGitExecCommand(t, repoPath, "git", "checkout", "-b", "feature-branch-2")
				if err := cmd.Run(); err != nil {
					t.Fatalf("Failed to create feature branch: %v", err)
				}

				// Create and commit a new file
				testFile := filepath.Join(repoPath, "amend-test.txt")
				if err := os.WriteFile(testFile, []byte("initial content\n"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}

				cmds := [][]string{
					{"git", "add", "amend-test.txt"},
					{"git", "commit", "-m", "Initial commit"},
				}
				for _, cmdArgs := range cmds {
					cmd := testGitExecCommand(t, repoPath, cmdArgs[0], cmdArgs[1:]...)
					if err := cmd.Run(); err != nil {
						t.Fatalf("Failed to setup commit: %v", err)
					}
				}

				// Modify the file so there's something to amend
				if err := os.WriteFile(testFile, []byte("modified content\n"), 0644); err != nil {
					t.Fatalf("Failed to modify test file: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := setupTestGitContext(t, reposPath)
			testViper := config.Viper(testCtx)

			if tt.message != "" {
				testViper.Set(config.CommitMessage, tt.message)
			}
			testViper.Set(config.CommitAmend, tt.amend)

			// Setup repository state
			for _, repo := range tt.repos {
				repoDir := filepath.Join(reposPath, "example.com", "test-project", repo)
				if tt.setupFunc != nil {
					tt.setupFunc(t, repoDir)
				}
			}

			cmd := addCommitCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			args := tt.repos
			if tt.message != "" {
				args = append([]string{"--message", tt.message}, args...)
			}
			if tt.amend {
				args = append([]string{"--amend"}, args...)
			}

			cmd.SetArgs(args)

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

func TestCommitCommandRunWithoutMessage(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1"})
	ctx := setupTestGitContext(t, reposPath)

	// Setup feature branch
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	gitCmd := testGitExecCommand(t, repoDir, "git", "checkout", "-b", "feature-branch")
	if err := gitCmd.Run(); err != nil {
		t.Fatalf("Failed to create feature branch: %v", err)
	}

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	err := cmd.ExecuteContext(ctx)
	if err == nil {
		t.Error("Expected error when commit message not provided for new commit")
	}
}

func TestCommitCommandRunOnMainBranch(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1"})
	ctx := setupTestGitContext(t, reposPath)
	viper := config.Viper(ctx)

	viper.Set(config.CommitMessage, "Test commit")

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--message", "Test commit", "repo-1"})

	// Should output error because ValidateBranch prevents committing on main branch
	_ = cmd.ExecuteContext(ctx)

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("ERROR")) {
		t.Errorf("Expected error output when trying to commit on main branch, got: %s", output)
	}
}

func TestUpdateCommandRun(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1", "repo-2"})

	tests := []struct {
		name           string
		repos          []string
		expectedOutput []string
		setupFunc      func(t *testing.T, repoPath string)
	}{
		{
			name:  "Update single repo",
			repos: []string{"repo-1"},
			expectedOutput: []string{
				"repo-1",
			},
			setupFunc: func(t *testing.T, repoPath string) {
				// Create and switch to feature branch
				cmd := testGitExecCommand(t, repoPath, "git", "checkout", "-b", "feature-branch")
				if err := cmd.Run(); err != nil {
					t.Fatalf("Failed to create feature branch: %v", err)
				}
			},
		},
		{
			name:  "Update multiple repos",
			repos: []string{"repo-1", "repo-2"},
			expectedOutput: []string{
				"repo-1",
				"repo-2",
			},
			setupFunc: func(t *testing.T, repoPath string) {
				// Create and switch to feature branch
				cmd := testGitExecCommand(t, repoPath, "git", "checkout", "-b", "another-branch")
				if err := cmd.Run(); err != nil {
					t.Fatalf("Failed to create feature branch: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx := setupTestGitContext(t, reposPath)

			// Setup repository state
			for _, repo := range tt.repos {
				repoDir := filepath.Join(reposPath, "example.com", "test-project", repo)
				if tt.setupFunc != nil {
					tt.setupFunc(t, repoDir)
				}
			}

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

func TestStatusCommandRun(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1", "repo-2"})

	tests := []struct {
		name           string
		repos          []string
		expectedOutput []string
	}{
		{
			name:  "Status single repo",
			repos: []string{"repo-1"},
			expectedOutput: []string{
				"repo-1",
			},
		},
		{
			name:  "Status multiple repos",
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

			cmd := addStatusCmd()

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

func TestDiffCommandRun(t *testing.T) {
	reposPath := setupTestGitRepos(t, []string{"repo-1"})

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
