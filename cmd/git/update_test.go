package git

import (
	"bytes"
	"path/filepath"
	"testing"

	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

func TestAddUpdateCmd(t *testing.T) {
	cmd := addUpdateCmd()

	if cmd == nil {
		t.Fatal("addUpdateCmd() returned nil")
	}

	if cmd.Use != "update <repository>..." {
		t.Errorf("Expected Use to be 'update <repository>...', got %s", cmd.Use)
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
