package git

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

func TestAddCommitCmd(t *testing.T) {
	cmd := addCommitCmd()

	if cmd == nil {
		t.Fatal("addCommitCmd() returned nil")
	}

	if cmd.Use != "commit <repository>..." {
		t.Errorf("Expected Use to be 'commit <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestCommitCmdFlags(t *testing.T) {
	cmd := addCommitCmd()

	// Test amend flag
	amendFlag := cmd.Flags().Lookup("amend")
	if amendFlag == nil {
		t.Fatal("amend flag not found")
	}

	if amendFlag.Shorthand != "a" {
		t.Errorf("Expected amend flag shorthand to be 'a', got %s", amendFlag.Shorthand)
	}

	// Test message flag
	messageFlag := cmd.Flags().Lookup("message")
	if messageFlag == nil {
		t.Fatal("message flag not found")
	}

	if messageFlag.Shorthand != "m" {
		t.Errorf("Expected message flag shorthand to be 'm', got %s", messageFlag.Shorthand)
	}
}

func TestCommitCmdArgs(t *testing.T) {
	cmd := addCommitCmd()

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

func TestCommitCmdPreRunE(t *testing.T) {
	cmd := addCommitCmd()

	// Test PreRunE function exists
	if cmd.PreRunE == nil {
		t.Error("Expected PreRunE function to be set")
		return
	}

	// Set the context on the command so PreRunE can access it
	ctx := loadFixture(t)
	cmd.SetContext(ctx)

	// Test with amend flag set (should not require message)
	cmd.Flags().Set("amend", "true")
	err := cmd.PreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("Expected no error when amend flag is set, got %v", err)
	}
}

func TestCommitCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"}, true)

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
			setupFunc: func(t *testing.T, repoPath string) {},
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
					testhelper.ExecCommand(t, repoPath, cmdArgs[0], cmdArgs[1:]...)
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
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	ctx := setupTestGitContext(t, reposPath)

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	if err := cmd.ExecuteContext(ctx); err == nil {
		t.Error("Expected error when commit message not provided for new commit")
	}
}

func TestCommitCommandRunOnMainBranch(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
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

func TestCommitCommandRunWithNoPushFlag(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	ctx := setupTestGitContext(t, reposPath)
	viper := config.Viper(ctx)

	viper.Set(config.CommitMessage, "Test commit")

	// Setup feature branch
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testhelper.ExecCommand(t, repoDir, "git", "checkout", "-b", "feature-branch")

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--message", "Test commit", "--no-push", "repo-1"})

	_ = cmd.ExecuteContext(ctx)
}

func TestCommitCommandRunWithAmendAndMessage(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	ctx := setupTestGitContext(t, reposPath)

	// Setup feature branch with a commit
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testhelper.ExecCommand(t, repoDir, "git", "checkout", "-b", "feature-branch")

	// Create and commit a file
	testFile := filepath.Join(repoDir, "test-amend.txt")
	if err := os.WriteFile(testFile, []byte("content\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmds := [][]string{
		{"git", "add", "test-amend.txt"},
		{"git", "commit", "-m", "Initial commit"},
	}
	for _, cmdArgs := range cmds {
		testhelper.ExecCommand(t, repoDir, cmdArgs[0], cmdArgs[1:]...)
	}

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified content\n"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--amend", "--message", "Amended commit", "--no-push", "repo-1"})

	_ = cmd.ExecuteContext(ctx)
}
