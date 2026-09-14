package git

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddCommitCmd(t *testing.T) {
	cmd := addCommitCmd()

	if cmd == nil {
		t.Fatal("addCommitCmd() returned nil")
	}

	expectedUse := "commit {-m <message>|--amend [-m <message>]} [-s <mode>] [--push] <repository>..."
	if cmd.Use != expectedUse {
		t.Errorf("Expected Use to be %q, got %s", expectedUse, cmd.Use)
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

	// git reads -a as --all, so --amend must not claim that shorthand
	if amendFlag.Shorthand != "" {
		t.Errorf("Expected amend flag to have no shorthand, got %s", amendFlag.Shorthand)
	}

	// Test message flag
	messageFlag := cmd.Flags().Lookup("message")
	if messageFlag == nil {
		t.Fatal("message flag not found")
	}

	if messageFlag.Shorthand != "m" {
		t.Errorf("Expected message flag shorthand to be 'm', got %s", messageFlag.Shorthand)
	}

	// Test stage flag
	stageFlag := cmd.Flags().Lookup("stage")
	if stageFlag == nil {
		t.Fatal("stage flag not found")
	}

	if stageFlag.Shorthand != "s" {
		t.Errorf("Expected stage flag shorthand to be 's', got %s", stageFlag.Shorthand)
	}

	if stageFlag.DefValue != StageAll {
		t.Errorf("Expected stage flag default to be %q, got %s", StageAll, stageFlag.DefValue)
	}

	// Test push flag pair. These are registered as persistent flags by
	// BuildBoolFlagsDefault, which Flags() only exposes once cobra merges them.
	local := cmd.LocalFlags()

	pushFlag := local.Lookup("push")
	if pushFlag == nil {
		t.Fatal("push flag not found")
	}

	if pushFlag.DefValue != "false" {
		t.Errorf("Expected push flag to default to false, got %s", pushFlag.DefValue)
	}

	if local.Lookup("no-push") == nil {
		t.Error("no-push flag not found")
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

// gitOutput runs a git command in dir and returns its trimmed stdout.
func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}

	return strings.TrimSpace(string(out))
}

// TestCommitStageModes verifies that each staging mode commits exactly the expected
// files. The fixture is built so all three modes produce a different result:
// staged.txt is already in the index, test.txt is a tracked but unstaged
// modification, and untracked.txt has never been added.
func TestCommitStageModes(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		expected []string
	}{
		{
			name:     "none commits only the index",
			mode:     StageNone,
			expected: []string{"staged.txt"},
		},
		{
			name:     "tracked adds unstaged modifications",
			mode:     StageTracked,
			expected: []string{"staged.txt", "test.txt"},
		},
		{
			name:     "all adds untracked files too",
			mode:     StageAll,
			expected: []string{"staged.txt", "test.txt", "untracked.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
			ctx := setupTestGitContext(t, reposPath)

			repoDir := filepath.Join(reposPath, "example.com", "test-project", "repo-1")

			// Already in the index
			if err := os.WriteFile(filepath.Join(repoDir, "staged.txt"), []byte("staged\n"), 0644); err != nil {
				t.Fatalf("Failed to create staged file: %v", err)
			}
			testhelper.ExecCommand(t, repoDir, "git", "add", "staged.txt")

			// Tracked (committed by SetupRepos) but modified without staging
			if err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("modified\n"), 0644); err != nil {
				t.Fatalf("Failed to modify tracked file: %v", err)
			}

			// Never added
			if err := os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("untracked\n"), 0644); err != nil {
				t.Fatalf("Failed to create untracked file: %v", err)
			}

			cmd := addCommitCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs([]string{"--stage", tt.mode, "--message", "Stage mode test", "repo-1"})

			if err := cmd.ExecuteContext(ctx); err != nil {
				t.Fatalf("Command execution failed: %v\n%s", err, buf.String())
			}

			committed := gitOutput(t, repoDir, "show", "--pretty=format:", "--name-only", "HEAD")

			var actual []string
			for _, line := range strings.Split(committed, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					actual = append(actual, line)
				}
			}

			sort.Strings(actual)

			if !slices.Equal(actual, tt.expected) {
				t.Errorf("Expected commit to contain %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestCommitStageInvalidMode(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	ctx := setupTestGitContext(t, reposPath)

	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--stage", "everything", "--message", "Test", "repo-1"})

	err := cmd.ExecuteContext(ctx)
	if err == nil {
		t.Fatal("Expected error for an invalid --stage value")
	}

	if !strings.Contains(err.Error(), config.GitCommitStage) {
		t.Errorf("Expected error to reference %s, got: %v", config.GitCommitStage, err)
	}
}

// TestCommitPushFlagPair verifies that --push and --no-push both resolve through
// the shared config key, and that they cannot be combined.
func TestCommitPushFlagPair(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{name: "default is no push", args: nil, expected: false},
		{name: "explicit push", args: []string{"--push"}, expected: true},
		{name: "explicit no-push", args: []string{"--no-push"}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			cmd := addCommitCmd()
			cmd.SetContext(ctx)

			// Parse flags without running, so the PreRunE bindings are exercised in isolation.
			if err := cmd.ParseFlags(append([]string{"--message", "Test"}, tt.args...)); err != nil {
				t.Fatalf("Failed parsing flags: %v", err)
			}

			if err := cmd.PreRunE(cmd, []string{"repo-1"}); err != nil {
				t.Fatalf("PreRunE failed: %v", err)
			}

			if got := config.Viper(ctx).GetBool(config.GitCommitPush); got != tt.expected {
				t.Errorf("Expected %s to be %v, got %v", config.GitCommitPush, tt.expected, got)
			}
		})
	}
}

func TestCommitPushFlagsMutuallyExclusive(t *testing.T) {
	ctx := loadFixture(t)
	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--push", "--no-push", "--message", "Test", "repo-1"})

	if err := cmd.ExecuteContext(ctx); err == nil {
		t.Fatal("Expected error when both --push and --no-push are set")
	}
}
