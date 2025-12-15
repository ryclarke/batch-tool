package pr

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
)

// setupTestRepos creates a temporary directory structure for git repositories
func setupTestRepos(t *testing.T, repos []string) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create directory structure: tmpDir/host/project/repo
	host := "example.com"
	project := "test-project"

	for _, repo := range repos {
		repoDir := filepath.Join(tmpDir, host, project, repo)
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatalf("Failed to create test repo directory: %v", err)
		}

		// Initialize a real git repository using git commands
		cmds := [][]string{
			{"git", "init"},
			{"git", "config", "user.email", "test@example.com"},
			{"git", "config", "user.name", "Test User"},
			{"git", "checkout", "-b", "main"},
			{"git", "commit", "--allow-empty", "-m", "Initial commit"},
			{"git", "checkout", "-b", "feature-branch"},
		}

		for _, cmdArgs := range cmds {
			cmd := testExecCommand(t, repoDir, cmdArgs[0], cmdArgs[1:]...)
			if err := cmd.Run(); err != nil {
				t.Fatalf("Failed to run git command %v: %v", cmdArgs, err)
			}
		}
	}

	return tmpDir
}

// testExecCommand creates an exec.Command with the given working directory
func testExecCommand(t *testing.T, dir string, name string, args ...string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd
}

// setupTestContext configures a test context with fake SCM provider
// Returns the provider instance that will be used by all scm.Get calls
func setupTestContext(t *testing.T, reposPath string) (context.Context, *fake.Fake) {
	t.Helper()

	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Use a unique provider name for each test to avoid state sharing
	providerName := "fake-test-" + t.Name()

	// Configure viper for testing
	viper.Set(config.GitProvider, providerName)
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.AuthToken, "fake-token")
	viper.Set(config.GitDirectory, reposPath)
	viper.Set(config.GitHost, "example.com")
	viper.Set(config.Branch, "feature-branch")
	viper.Set(config.MaxConcurrency, 1) // Serial execution for predictable test output
	viper.Set(config.ChannelBuffer, 10)

	// Create test repositories
	testRepos := []*scm.Repository{
		{
			Name:          "repo-1",
			Description:   "Test Repository 1",
			Project:       "test-project",
			DefaultBranch: "main",
			Labels:        []string{"test"},
		},
		{
			Name:          "repo-2",
			Description:   "Test Repository 2",
			Project:       "test-project",
			DefaultBranch: "main",
			Labels:        []string{"test"},
		},
	}

	// Create a single provider instance that will be shared
	sharedProvider := fake.NewFake("test-project", testRepos)

	// Register fake provider factory that returns the same instance each time
	scm.Register(providerName, func(ctx context.Context, project string) scm.Provider {
		return sharedProvider
	})

	return ctx, sharedProvider
}

func TestNewCommandRun(t *testing.T) {
	// Set up test repositories
	reposPath := setupTestRepos(t, []string{"repo-1", "repo-2"})

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
	reposPath := setupTestRepos(t, []string{"repo-1"})
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

func TestEditCommandRun(t *testing.T) {
	reposPath := setupTestRepos(t, []string{"repo-1", "repo-2"})

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
			_, err := testProvider.OpenPullRequest("repo-1", "feature-branch", "Original Title", "Original Description", []string{"original-reviewer"})
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

func TestMergeCommandRun(t *testing.T) {
	reposPath := setupTestRepos(t, []string{"repo-1", "repo-2"})

	tests := []struct {
		name           string
		repos          []string
		expectedOutput []string
	}{
		{
			name:  "Merge single PR",
			repos: []string{"repo-1"},
			expectedOutput: []string{
				"Merged pull request",
				"Test Title",
			},
		},
		{
			name:  "Merge multiple PRs",
			repos: []string{"repo-1", "repo-2"},
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
				_, err := testProvider.OpenPullRequest(repo, "feature-branch", "Test Title", "Test Description", []string{"reviewer1"})
				if err != nil {
					t.Fatalf("Failed to create test PR for %s: %v", repo, err)
				}
			}

			cmd := addMergeCmd()

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

func TestEditCommandRunPRNotFound(t *testing.T) {
	reposPath := setupTestRepos(t, []string{"repo-1"})
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

func TestMergeCommandRunPRNotFound(t *testing.T) {
	reposPath := setupTestRepos(t, []string{"repo-1"})
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
