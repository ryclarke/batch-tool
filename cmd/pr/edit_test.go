package pr

import (
	"bytes"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddEditCmd(t *testing.T) {
	cmd := addEditCmd()

	if cmd == nil {
		t.Fatal("addEditCmd() returned nil")
	}

	if cmd.Use != "edit <repository>..." {
		t.Errorf("Expected Use to be 'edit <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestEditCmdFlags(t *testing.T) {
	cmd := addEditCmd()

	// Test reset-reviewers flag
	resetReviewersFlag := cmd.Flags().Lookup("reset-reviewers")
	if resetReviewersFlag == nil {
		t.Error("reset-reviewers flag not found")
	}
}

func TestEditCmdArgs(t *testing.T) {
	cmd := addEditCmd()

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

func TestEditCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"}, true)

	tests := []struct {
		name           string
		repos          []string
		resetReviewers bool
		expectedOutput []string
	}{
		{
			name:           "Edit PR appending reviewers",
			repos:          []string{"repo-1"},
			resetReviewers: false,
			expectedOutput: []string{
				"Updated pull request",
				"Updated PR Title",
			},
		},
		{
			name:           "Edit PR resetting reviewers",
			repos:          []string{"repo-1"},
			resetReviewers: true,
			expectedOutput: []string{
				"Updated pull request",
				"Updated PR Title",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh context for each test
			testCtx, testProvider := setupTestContext(t, reposPath)
			testViper := config.Viper(testCtx)

			testViper.Set(config.PrTitle, "Updated PR Title")
			testViper.Set(config.PrDescription, "Updated PR Description")
			testViper.Set(config.PrReviewers, []string{"reviewer1", "reviewer2"})
			testViper.Set(config.PrResetReviewers, tt.resetReviewers)

			// Create PR using the provider
			_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Original Title", Description: "Original Description", Reviewers: []string{"original-reviewer"}})
			if err != nil {
				t.Fatalf("Failed to create test PR: %v", err)
			}

			cmd := addEditCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.repos)

			if tt.resetReviewers {
				cmd.Flags().Set("reset-reviewers", "true")
			}

			err = cmd.ExecuteContext(testCtx)
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

func TestEditCommandRunPRNotFound(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	ctx, _ := setupTestContext(t, reposPath)
	viper := config.Viper(ctx)

	viper.Set(config.PrTitle, "Updated PR Title")
	viper.Set(config.PrDescription, "Updated PR Description")
	viper.Set(config.PrReviewers, []string{"reviewer1"})

	// Don't create a PR - the edit command should report an error

	cmd := addEditCmd()

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
