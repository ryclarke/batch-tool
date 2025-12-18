package git

import (
	"bytes"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddBranchCmd(t *testing.T) {
	cmd := addBranchCmd()

	if cmd == nil {
		t.Fatal("addBranchCmd() returned nil")
	}

	if cmd.Use != "branch <repository>..." {
		t.Errorf("Expected Use to be 'branch <repository>...', got %s", cmd.Use)
	}

	// Test aliases
	aliases := cmd.Aliases
	if len(aliases) != 1 || aliases[0] != "checkout" {
		t.Errorf("Expected aliases to contain 'checkout', got %v", aliases)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestBranchCmdFlags(t *testing.T) {
	cmd := addBranchCmd()

	// Test branch flag
	branchFlag := cmd.Flags().Lookup("branch")
	if branchFlag == nil {
		t.Fatal("branch flag not found")
	}

	if branchFlag.Shorthand != "b" {
		t.Errorf("Expected branch flag shorthand to be 'b', got %s", branchFlag.Shorthand)
	}
}

func TestBranchCmdArgs(t *testing.T) {
	cmd := addBranchCmd()

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

func TestBranchCmdPreRunE(t *testing.T) {
	ctx := loadFixture(t)
	cmd := addBranchCmd()
	cmd.SetContext(ctx)

	// Test PreRunE function exists
	if cmd.PreRunE == nil {
		t.Error("Expected PreRunE function to be set")
		return
	}

	// Test without branch flag (should require branch)
	err := cmd.PreRunE(cmd, []string{})
	if err == nil {
		t.Error("Expected error when branch flag is not set")
	}
}

func TestBranchCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"})

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
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})
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

func TestBranchCommandWithSpecialCharacters(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"})

	testCtx := setupTestGitContext(t, reposPath)
	testViper := config.Viper(testCtx)
	branchName := "feature/test-branch"
	testViper.Set(config.Branch, branchName)

	cmd := addBranchCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--branch", branchName, "repo-1"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("repo-1")) {
		t.Errorf("Expected output to contain 'repo-1', got: %s", output)
	}
}
