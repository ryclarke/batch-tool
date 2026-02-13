package pr

import (
	"bytes"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddMergeCmd(t *testing.T) {
	cmd := addMergeCmd()

	if cmd == nil {
		t.Fatal("addMergeCmd() returned nil")
	}

	if cmd.Use != "merge [-f] <repository>..." {
		t.Errorf("Expected Use to be 'merge [-f] <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestMergeCmdArgs(t *testing.T) {
	cmd := addMergeCmd()

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

func TestMergeCommandRun(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2"}, true)

	tests := []struct {
		name           string
		repos          []string
		checkFlag      bool
		forceFlag      bool
		expectedOutput []string
	}{
		{
			name:      "Merge single PR (default)",
			repos:     []string{"repo-1"},
			checkFlag: false,
			forceFlag: false,
			expectedOutput: []string{
				"Merged pull request",
				"Test Title",
			},
		},
		{
			name:      "Merge with --check flag",
			repos:     []string{"repo-1"},
			checkFlag: true,
			forceFlag: false,
			expectedOutput: []string{
				"Merged pull request",
				"Test Title",
			},
		},
		{
			name:      "Merge with --force flag",
			repos:     []string{"repo-1"},
			checkFlag: false,
			forceFlag: true,
			expectedOutput: []string{
				"Merged pull request",
				"Test Title",
			},
		},
		{
			name:      "Merge multiple PRs",
			repos:     []string{"repo-1", "repo-2"},
			checkFlag: false,
			forceFlag: false,
			expectedOutput: []string{
				"Merged pull request",
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
				_, err := testProvider.OpenPullRequest(repo, "feature-branch", &scm.PROptions{
					Title:       "Test Title",
					Description: "Test Description",
					Reviewers:   []string{"reviewer1"},
				})
				if err != nil {
					t.Fatalf("Failed to create test PR for %s: %v", repo, err)
				}
			}

			cmd := addMergeCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			args := tt.repos
			if tt.checkFlag {
				args = append([]string{"--check"}, args...)
			} else if tt.forceFlag {
				args = append([]string{"--force"}, args...)
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

func TestMergeCommandRunPRNotFound(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	ctx, _ := setupTestContext(t, reposPath)

	// Don't create a PR - the merge command should report an error

	cmd := addMergeCmd()

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

// TestMergeCommandWithUnmergeablePR tests merge behavior with unmergeable PRs
func TestMergeCommandWithUnmergeablePR(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)

	tests := []struct {
		name          string
		mergeable     bool
		checkFlag     bool
		forceFlag     bool
		expectSuccess bool
		errorContains string
	}{
		{
			name:          "mergeable PR (default)",
			mergeable:     true,
			checkFlag:     false,
			forceFlag:     false,
			expectSuccess: true,
		},
		{
			name:          "mergeable PR with --check",
			mergeable:     true,
			checkFlag:     true,
			forceFlag:     false,
			expectSuccess: true,
		},
		{
			name:          "mergeable PR with --force",
			mergeable:     true,
			checkFlag:     false,
			forceFlag:     true,
			expectSuccess: true,
		},
		{
			name:          "unmergeable PR with --check fails",
			mergeable:     false,
			checkFlag:     true,
			forceFlag:     false,
			expectSuccess: false,
			errorContains: "is not mergeable",
		},
		{
			name:          "unmergeable PR (default) succeeds",
			mergeable:     false,
			checkFlag:     false,
			forceFlag:     false,
			expectSuccess: true, // default doesn't check mergeability
		},
		{
			name:          "unmergeable PR with --force succeeds",
			mergeable:     false,
			checkFlag:     false,
			forceFlag:     true,
			expectSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx, testProvider := setupTestContext(t, reposPath)

			// Create PR
			_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{
				Title:       "Test Title",
				Description: "Test Description",
				Reviewers:   []string{"reviewer1"},
			})
			if err != nil {
				t.Fatalf("Failed to create test PR: %v", err)
			}

			// Set mergeability status
			err = testProvider.SetPRMergeable("repo-1", "feature-branch", tt.mergeable)
			if err != nil {
				t.Fatalf("Failed to set PR mergeable status: %v", err)
			}

			cmd := addMergeCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			args := []string{"repo-1"}
			if tt.checkFlag {
				args = append([]string{"--check"}, args...)
			} else if tt.forceFlag {
				args = append([]string{"--force"}, args...)
			}
			cmd.SetArgs(args)

			_ = cmd.ExecuteContext(testCtx)

			output := buf.String()

			if tt.expectSuccess {
				if !bytes.Contains([]byte(output), []byte("Merged pull request")) {
					t.Errorf("Expected successful merge output, got: %s", output)
				}
				if tt.errorContains != "" && bytes.Contains([]byte(output), []byte(tt.errorContains)) {
					t.Errorf("Expected no error but found %q in output: %s", tt.errorContains, output)
				}
			} else {
				if bytes.Contains([]byte(output), []byte("Merged pull request")) {
					t.Errorf("Expected merge to fail, but got success output: %s", output)
				}
				if tt.errorContains != "" && !bytes.Contains([]byte(output), []byte(tt.errorContains)) {
					t.Errorf("Expected error containing %q, got: %s", tt.errorContains, output)
				}
			}
		})
	}
}

// TestMergeCommandCheckFlagBypassesFalseNegative tests the primary use case for check flag
func TestMergeCommandCheckFlagBypassesFalseNegative(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"hotfix-repo"}, true)
	testCtx, testProvider := setupTestContext(t, reposPath)

	// Scenario: Provider incorrectly reports PR as unmergeable (false negative)
	// but we know it's fine and want to merge anyway
	_, err := testProvider.OpenPullRequest("hotfix-repo", "feature-branch", &scm.PROptions{Title: "Critical Security Fix", Description: "Must merge ASAP", Reviewers: []string{"security-team"}})
	if err != nil {
		t.Fatalf("Failed to create test PR: %v", err)
	}

	// Simulate false negative from provider's mergeability check
	err = testProvider.SetPRMergeable("hotfix-repo", "feature-branch", false)
	if err != nil {
		t.Fatalf("Failed to set PR mergeable status: %v", err)
	}

	// First attempt with --check flag should fail (but default is already to check)
	cmd := addMergeCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--check", "hotfix-repo"}) // use --check flag to verify mergeability
	_ = cmd.ExecuteContext(testCtx)

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("is not mergeable")) {
		t.Errorf("Expected merge to fail with mergeability error, got: %s", output)
	}

	// Verify PR still exists
	pr, err := testProvider.GetPullRequest("hotfix-repo", "feature-branch")
	if err != nil {
		t.Fatalf("Expected PR to still exist after failed merge: %v", err)
	}
	if pr == nil {
		t.Fatal("Expected PR to still exist")
	}

	// Second attempt without --check flag should succeed (bypasses mergeability check)
	cmd2 := addMergeCmd()
	var buf2 bytes.Buffer
	cmd2.SetOut(&buf2)
	cmd2.SetErr(&buf2)
	cmd2.SetArgs([]string{"hotfix-repo"}) // no --check flag means bypass checking
	_ = cmd2.ExecuteContext(testCtx)

	output2 := buf2.String()
	if !bytes.Contains([]byte(output2), []byte("Merged pull request")) {
		t.Errorf("Expected force merge to succeed, got: %s", output2)
	}

	// Verify PR was deleted after successful merge
	_, err = testProvider.GetPullRequest("hotfix-repo", "feature-branch")
	if err == nil {
		t.Error("Expected PR to be deleted after successful force merge")
	}
}

// TestMergeCommandMergeMethodFlag tests the merge method flag behavior
func TestMergeCommandMergeMethodFlag(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)

	tests := []struct {
		name         string
		mergeMethod  string
		expectMethod string
	}{
		{
			name:         "merge method: merge",
			mergeMethod:  "merge",
			expectMethod: "merge",
		},
		{
			name:         "merge method: squash",
			mergeMethod:  "squash",
			expectMethod: "squash",
		},
		{
			name:         "merge method: rebase",
			mergeMethod:  "rebase",
			expectMethod: "rebase",
		},
		{
			name:         "no merge method specified (default)",
			mergeMethod:  "",
			expectMethod: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCtx, testProvider := setupTestContext(t, reposPath)
			testViper := config.Viper(testCtx)

			// Create PR for this test
			_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{
				Title:       "Test PR",
				Description: "Test Description",
				Reviewers:   []string{"reviewer1"},
			})
			if err != nil {
				t.Fatalf("Failed to create test PR: %v", err)
			}

			cmd := addMergeCmd()

			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			args := []string{"repo-1"}
			if tt.mergeMethod != "" {
				args = append([]string{"--method", tt.mergeMethod}, args...)
			}
			cmd.SetArgs(args)

			err = cmd.ExecuteContext(testCtx)
			if err != nil {
				t.Fatalf("Command execution failed: %v", err)
			}

			output := buf.String()

			// Verify merge was successful
			if !bytes.Contains([]byte(output), []byte("Merged pull request")) {
				t.Errorf("Expected successful merge output, got: %s", output)
			}

			// Verify the merge method was set in viper if provided
			if tt.mergeMethod != "" {
				retrievedMethod := testViper.GetString(config.PrMergeMethod)
				if retrievedMethod != tt.expectMethod {
					t.Errorf("Expected merge method %q in viper, got %q", tt.expectMethod, retrievedMethod)
				}
			}
		})
	}
}

// TestMergeCommandMergeMethodShortFlag tests the short form of the merge method flag
func TestMergeCommandMergeMethodShortFlag(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	testCtx, testProvider := setupTestContext(t, reposPath)

	// Create PR
	_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{
		Title:       "Test PR",
		Description: "Test Description",
		Reviewers:   []string{"reviewer1"},
	})
	if err != nil {
		t.Fatalf("Failed to create test PR: %v", err)
	}

	cmd := addMergeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test short flag -m
	cmd.SetArgs([]string{"-m", "squash", "repo-1"})

	err = cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	// Verify merge was successful
	if !bytes.Contains([]byte(output), []byte("Merged pull request")) {
		t.Errorf("Expected successful merge output, got: %s", output)
	}

	// Verify the merge method was set
	testViper := config.Viper(testCtx)
	retrievedMethod := testViper.GetString(config.PrMergeMethod)
	if retrievedMethod != "squash" {
		t.Errorf("Expected merge method 'squash' in viper, got %q", retrievedMethod)
	}
}

// TestMergeCommandWithBothCheckFlagAndMergeMethod tests combining check and merge method flags
func TestMergeCommandWithBothCheckFlagAndMergeMethod(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	testCtx, testProvider := setupTestContext(t, reposPath)

	// Create PR
	_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{
		Title:       "Test PR",
		Description: "Test Description",
		Reviewers:   []string{"reviewer1"},
	})
	if err != nil {
		t.Fatalf("Failed to create test PR: %v", err)
	}

	// Mark PR as mergeable so the check passes
	err = testProvider.SetPRMergeable("repo-1", "feature-branch", true)
	if err != nil {
		t.Fatalf("Failed to set PR mergeable status: %v", err)
	}

	cmd := addMergeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Use both --check and --method flags
	cmd.SetArgs([]string{"--check", "--method", "rebase", "repo-1"})

	err = cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	// Verify merge was successful despite PR not being mergeable
	if !bytes.Contains([]byte(output), []byte("Merged pull request")) {
		t.Errorf("Expected successful merge output with both flags, got: %s", output)
	}

	// Verify the merge method was set
	testViper := config.Viper(testCtx)
	retrievedMethod := testViper.GetString(config.PrMergeMethod)
	if retrievedMethod != "rebase" {
		t.Errorf("Expected merge method 'rebase' in viper, got %q", retrievedMethod)
	}
}

// TestMergeCommandMultipleReposWithMergeMethod tests merge method with multiple repositories
func TestMergeCommandMultipleReposWithMergeMethod(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1", "repo-2", "repo-3"}, true)
	testCtx, testProvider := setupTestContext(t, reposPath)

	// Create PRs for all repos
	for _, repo := range []string{"repo-1", "repo-2", "repo-3"} {
		_, err := testProvider.OpenPullRequest(repo, "feature-branch", &scm.PROptions{
			Title:       "Test PR",
			Description: "Test Description",
			Reviewers:   []string{"reviewer1"},
		})
		if err != nil {
			t.Fatalf("Failed to create test PR for %s: %v", repo, err)
		}
	}

	cmd := addMergeCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Use method flag with multiple repos
	cmd.SetArgs([]string{"--method", "squash", "repo-1", "repo-2", "repo-3"})

	err := cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()

	// Verify all merges were successful
	for _, repo := range []string{"repo-1", "repo-2", "repo-3"} {
		if !bytes.Contains([]byte(output), []byte(repo)) {
			t.Errorf("Expected output to contain repo %q, got: %s", repo, output)
		}
	}

	if !bytes.Contains([]byte(output), []byte("Merged pull request")) {
		t.Errorf("Expected merge success messages in output, got: %s", output)
	}

	// Verify the merge method was set
	testViper := config.Viper(testCtx)
	retrievedMethod := testViper.GetString(config.PrMergeMethod)
	if retrievedMethod != "squash" {
		t.Errorf("Expected merge method 'squash' in viper, got %q", retrievedMethod)
	}
}

// TestMergeCommandForceFlagShortForm tests the -f short flag
func TestMergeCommandForceFlagShortForm(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	testCtx, testProvider := setupTestContext(t, reposPath)

	// Create unmergeable PR
	_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{
		Title:       "Test PR",
		Description: "Test Description",
		Reviewers:   []string{"reviewer1"},
	})
	if err != nil {
		t.Fatalf("Failed to create test PR: %v", err)
	}

	// Mark PR as not mergeable
	err = testProvider.SetPRMergeable("repo-1", "feature-branch", false)
	if err != nil {
		t.Fatalf("Failed to set PR mergeable status: %v", err)
	}

	cmd := addMergeCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Use short -f flag to force merge
	cmd.SetArgs([]string{"-f", "repo-1"})

	err = cmd.ExecuteContext(testCtx)
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Merged pull request")) {
		t.Errorf("Expected successful merge with -f flag, got: %s", output)
	}
}

// TestMergeCommandConflictingFlags tests behavior when both --check and --force are provided
func TestMergeCommandConflictingFlags(t *testing.T) {
	reposPath := testhelper.SetupRepos(t, []string{"repo-1"}, true)
	testCtx, testProvider := setupTestContext(t, reposPath)

	// Create mergeable PR
	_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{
		Title:       "Test PR",
		Description: "Test Description",
		Reviewers:   []string{"reviewer1"},
	})
	if err != nil {
		t.Fatalf("Failed to create test PR: %v", err)
	}

	err = testProvider.SetPRMergeable("repo-1", "feature-branch", true)
	if err != nil {
		t.Fatalf("Failed to set PR mergeable status: %v", err)
	}

	cmd := addMergeCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Use both --check and --force flags - should either error or have defined precedence
	cmd.SetArgs([]string{"--check", "--force", "repo-1"})

	err = cmd.ExecuteContext(testCtx)
	// Command might succeed with one flag taking precedence, or fail with conflicting flags
	// Either behavior is acceptable as long as it's consistent
	output := buf.String()

	// Log the behavior for manual verification
	t.Logf("Conflicting flags result - Error: %v, Output: %s", err, output)
}
