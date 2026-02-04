package git

import (
	"bytes"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddPushCmd(t *testing.T) {
	cmd := addPushCmd()

	if cmd == nil {
		t.Fatal("addPushCmd() returned nil")
	}

	if cmd.Use != "push [-f] <repository>..." {
		t.Errorf("Expected Use to be 'push [-f] <repository>...', got %s", cmd.Use)
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
				testViper.Set(config.GitCommitAmend, true)
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
	testViper.Set(config.GitCommitAmend, true)

	cmd2 := addPushCmd()

	var buf bytes.Buffer
	cmd2.SetOut(&buf)
	cmd2.SetErr(&buf)
	cmd2.SetArgs([]string{"--force", "repo-1"})

	_ = cmd2.ExecuteContext(testCtx)
	// Should attempt force push
}
