package pr

import (
	"bytes"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddNewCmd(t *testing.T) {
	cmd := addNewCmd()

	if cmd == nil {
		t.Fatal("addNewCmd() returned nil")
	}

	if cmd.Use != "new [--draft] [-t <title>] [-d <description>] [-r <reviewer>]... [-a] [-b <base-branch>] <repository>..." {
		t.Errorf("Expected Use to be 'new [--draft] [-t <title>] [-d <description>] [-r <reviewer>]... [-a] [-b <base-branch>] <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestNewCmdFlags(t *testing.T) {
	cmd := addNewCmd()

	// Test all-reviewers flag
	allReviewersFlag := cmd.Flags().Lookup("all-reviewers")
	if allReviewersFlag == nil {
		t.Fatal("all-reviewers flag not found")
	}

	if allReviewersFlag.Shorthand != "a" {
		t.Errorf("Expected all-reviewers flag shorthand to be 'a', got %s", allReviewersFlag.Shorthand)
	}
}

func TestNewCmdArgs(t *testing.T) {
	cmd := addNewCmd()

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

func TestNewCommandRun(t *testing.T) {
	// Set up test repositories
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"}, true)

	tests := []struct {
		name           string
		repos          []string
		allReviewers   bool
		expectedOutput []string
	}{
		{
			name:         "New PR with single reviewer",
			repos:        []string{"repo-1"},
			allReviewers: false,
			expectedOutput: []string{
				"New pull request",
				"feature-branch",
				"reviewer1",
			},
		},
		{
			name:         "New PR with all reviewers",
			repos:        []string{"repo-1"},
			allReviewers: true,
			expectedOutput: []string{
				"New pull request",
				"feature-branch",
				"reviewer1",
				"reviewer2",
			},
		},
		{
			name:         "New PR for multiple repos",
			repos:        []string{"repo-1", "repo-2"},
			allReviewers: false,
			expectedOutput: []string{
				"New pull request",
				"repo-1",
				"repo-2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up fresh context for each test
			testCtx, _ := setupTestContext(t, reposPath)
			testViper := config.Viper(testCtx)

			testViper.Set(config.PrTitle, "Test PR Title")
			testViper.Set(config.PrDescription, "Test PR Description")
			testViper.Set(config.PrReviewers, []string{"reviewer1", "reviewer2"})
			testViper.Set(config.PrAllReviewers, tt.allReviewers)

			cmd := addNewCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tt.repos)

			if tt.allReviewers {
				cmd.Flags().Set("all-reviewers", "true")
			}

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

func TestNewCommandRunWithoutReviewers(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	ctx, _ := setupTestContext(t, reposPath)
	viper := config.Viper(ctx)

	// Configure PR settings without reviewers
	viper.Set(config.PrTitle, "Test PR Title")
	viper.Set(config.PrDescription, "Test PR Description")
	viper.Set(config.PrReviewers, []string{})

	cmd := addNewCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"repo-1"})

	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("New pull request")) {
		t.Errorf("Expected output to contain 'New pull request', got: %s", output)
	}
}

func TestLookupReviewers(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Test with command-line reviewers (always returns all, regardless of allReviewers flag)
	viper.Set(config.PrReviewers, []string{"reviewer1", "reviewer2"})
	viper.Set(config.PrAllReviewers, false) // Flag doesn't affect manually-provided reviewers
	reviewers := lookupReviewers(ctx, "test-repo")

	if len(reviewers) != 2 {
		t.Errorf("Expected 2 manually-provided reviewers, got %d", len(reviewers))
	}
	if reviewers[0] != "reviewer1" || reviewers[1] != "reviewer2" {
		t.Errorf("Expected [reviewer1, reviewer2], got %v", reviewers)
	}

	// Test with default reviewers - limited to first unless allReviewers is set
	viper.Set(config.PrReviewers, []string{}) // Clear command-line reviewers
	defaultReviewers := map[string][]string{
		"test-repo":  {"default1", "default2"},
		"other-repo": {"other1"},
	}
	viper.Set(config.DefaultReviewers, defaultReviewers)
	viper.Set(config.PrAllReviewers, false)

	reviewers = lookupReviewers(ctx, "test-repo")
	if len(reviewers) != 1 {
		t.Errorf("Expected 1 default reviewer (limited), got %d", len(reviewers))
	}
	if reviewers[0] != "default1" {
		t.Errorf("Expected [default1], got %v", reviewers)
	}

	// Test with default reviewers and allReviewers flag set
	viper.Set(config.PrAllReviewers, true)
	reviewers = lookupReviewers(ctx, "test-repo")
	if len(reviewers) != 2 {
		t.Errorf("Expected 2 default reviewers (with allReviewers flag), got %d", len(reviewers))
	}
	if reviewers[0] != "default1" || reviewers[1] != "default2" {
		t.Errorf("Expected [default1, default2], got %v", reviewers)
	}

	// Test with non-existent repository
	reviewers = lookupReviewers(ctx, "nonexistent-repo")
	if len(reviewers) != 0 {
		t.Errorf("Expected 0 reviewers for nonexistent repo, got %d", len(reviewers))
	}
}
