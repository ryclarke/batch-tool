package pr

import (
	"bytes"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

func TestAddGetCmd(t *testing.T) {
	cmd := addGetCmd()

	if cmd == nil {
		t.Fatal("addGetCmd() returned nil")
	}

	if cmd.Use != "get <repository>..." {
		t.Errorf("Expected Use to be 'get <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	// Test aliases
	aliases := cmd.Aliases
	if len(aliases) != 1 || aliases[0] != "list" {
		t.Errorf("Expected aliases to contain 'list', got %v", aliases)
	}
}

func TestGetCmdArgs(t *testing.T) {
	cmd := addGetCmd()

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

func TestGetCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"}, true)

	tests := []struct {
		name           string
		repos          []string
		expectedOutput []string
	}{
		{
			name:  "Get single PR",
			repos: []string{"repo-1"},
			expectedOutput: []string{
				"(PR #",
				"Test Title",
			},
		},
		{
			name:  "Get multiple PRs",
			repos: []string{"repo-1", "repo-2"},
			expectedOutput: []string{
				"(PR #",
				"repo-1",
				"repo-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh context for each test
			testCtx, testProvider := setupTestContext(t, reposPath)
			testViper := config.Viper(testCtx)

			testViper.Set(config.PrTitle, "Test PR Title")
			testViper.Set(config.PrDescription, "Test PR Description")

			// Create PRs for this test using the provider
			for _, repo := range tt.repos {
				_, err := testProvider.OpenPullRequest(repo, "feature-branch", "Test Title", "Test Description", []string{"reviewer1"})
				if err != nil {
					t.Fatalf("Failed to create test PR for %s: %v", repo, err)
				}
			}

			cmd := addGetCmd()

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

func TestGetCommandRunPRNotFound(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	ctx, _ := setupTestContext(t, reposPath)

	// Don't create a PR - the get command should report an error

	cmd := addGetCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	// The command itself doesn't return an error, but prints it to output
	_ = cmd.ExecuteContext(ctx)

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("pull request not found")) {
		t.Errorf("Expected error message to contain 'pull request not found', got: %s", output)
	}
}
