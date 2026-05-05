package git

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddCommitCmd(t *testing.T) {
	cmd := addCommitCmd()

	if cmd == nil {
		t.Fatal("addCommitCmd() returned nil")
	}

	if cmd.Use != "commit {-m <message>|--amend [-m <message>]} [--push] <repository>..." {
		t.Errorf("Expected Use to be 'commit {-m <message>|--amend [-m <message>]} [--push] <repository>...', got %s", cmd.Use)
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

func TestCommitCmdPreRunEAmendPushSetsForce(t *testing.T) {
	cmd := addCommitCmd()
	ctx := loadFixture(t)
	cmd.SetContext(ctx)

	viper := config.Viper(ctx)
	viper.Set(config.GitPushForce, false)
	viper.Set(config.GitCommitPush, true)

	if err := cmd.Flags().Set("amend", "true"); err != nil {
		t.Fatalf("Failed setting amend flag: %v", err)
	}

	if err := cmd.PreRunE(cmd, []string{}); err != nil {
		t.Fatalf("Expected no error when amend+push are set, got %v", err)
	}

	if !viper.GetBool(config.GitPushForce) {
		t.Fatal("Expected GitPushForce to be true when both amend and push are true")
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
			setupFunc: func(t *testing.T, repoPath string) {
				testFile := filepath.Join(repoPath, "commit-test.txt")
				if err := os.WriteFile(testFile, []byte("new content\n"), 0644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
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
				testViper.Set(config.GitCommitMessage, tt.message)
			}
			testViper.Set(config.GitCommitAmend, tt.amend)

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

	viper.Set(config.GitCommitMessage, "Test commit")

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--message", "Test commit", "repo-1"})

	// Should return an error because ValidateBranch prevents committing on main branch
	err := cmd.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("Expected error when trying to commit on main branch")
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("ERROR")) {
		t.Errorf("Expected error output when trying to commit on main branch, got: %s", output)
	}
}

func TestCommitCommandRunWithNoPushFlag(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
	ctx := setupTestGitContext(t, reposPath)
	viper := config.Viper(ctx)

	viper.Set(config.GitCommitMessage, "Test commit")

	// Setup feature branch
	repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")
	testhelper.ExecCommand(t, repoDir, "git", "checkout", "-b", "feature-branch")

	// Create a change so a non-amend commit succeeds.
	testFile := filepath.Join(repoDir, "commit-no-push.txt")
	if err := os.WriteFile(testFile, []byte("commit without push\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--message", "Test commit", "repo-1"})

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Fatalf("Expected no error when committing on feature branch without push, got: %v", err)
	}
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
	cmd.SetArgs([]string{"--amend", "--message", "Amended commit", "repo-1"})

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Fatalf("Expected no error for amend with message on feature branch, got: %v", err)
	}
}
