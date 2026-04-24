package pr

import (
	"bytes"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestAddNewCmd(t *testing.T) {
	if addNewCmd() == nil {
		t.Fatal("addNewCmd() returned nil")
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
		expectedOutput []string
	}{
		{
			name:  "New PR with reviewers",
			repos: []string{"repo-1"},
			expectedOutput: []string{
				"New pull request",
				"feature-branch",
				"reviewer1",
				"reviewer2",
			},
		},
		{
			name:  "New PR for multiple repos",
			repos: []string{"repo-1", "repo-2"},
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

			cmd := addNewCmd()

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

	// CLI-provided reviewers always take precedence
	viper.Set(config.PrReviewers, []string{"reviewer1", "reviewer2"})
	reviewers := lookupReviewers(ctx, "test-repo")

	if len(reviewers) != 2 {
		t.Errorf("Expected 2 manually-provided reviewers, got %d", len(reviewers))
	}

	// Default reviewers — all returned
	viper.Set(config.PrReviewers, []string{})
	defaultReviewers := map[string][]string{
		"test-repo":  {"default1", "default2"},
		"other-repo": {"other1"},
	}
	viper.Set(config.DefaultReviewers, defaultReviewers)

	reviewers = lookupReviewers(ctx, "test-repo")
	if len(reviewers) != 2 {
		t.Errorf("Expected 2 default reviewers, got %d", len(reviewers))
	}

	// Non-existent repository returns empty
	reviewers = lookupReviewers(ctx, "nonexistent-repo")
	if len(reviewers) != 0 {
		t.Errorf("Expected 0 reviewers for nonexistent repo, got %d", len(reviewers))
	}
}

func TestLookupReviewersWithLabels(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Populate catalog labels so GetLabelsForRepo can find them
	catalog.Labels["backend"] = mapset.NewSet("test-repo", "other-repo")
	catalog.Labels["shared"] = mapset.NewSet("test-repo")
	t.Cleanup(func() {
		delete(catalog.Labels, "backend")
		delete(catalog.Labels, "shared")
	})

	viper.Set(config.PrReviewers, []string{})
	viper.Set(config.DefaultReviewers, map[string][]string{
		"test-repo": {"repo-reviewer"},
		"~backend":  {"backend-reviewer1", "backend-reviewer2"},
		"~shared":   {"shared-reviewer"},
	})
	viper.Set(config.TokenLabel, "~")

	reviewers := lookupReviewers(ctx, "test-repo")

	// Should include: repo-specific + backend label + shared label (deduped)
	got := make(map[string]bool)
	for _, r := range reviewers {
		got[r] = true
	}

	for _, want := range []string{"repo-reviewer", "backend-reviewer1", "backend-reviewer2", "shared-reviewer"} {
		if !got[want] {
			t.Errorf("Expected reviewer %q in results, got %v", want, reviewers)
		}
	}
}

func TestLookupReviewersDeduplication(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	catalog.Labels["team"] = mapset.NewSet("test-repo")
	t.Cleanup(func() { delete(catalog.Labels, "team") })

	viper.Set(config.PrReviewers, []string{})
	viper.Set(config.DefaultReviewers, map[string][]string{
		"test-repo": {"alice", "bob"},
		"~team":     {"bob", "carol"}, // "bob" appears in both
	})
	viper.Set(config.TokenLabel, "~")

	reviewers := lookupReviewers(ctx, "test-repo")

	bobCount := 0
	for _, r := range reviewers {
		if r == "bob" {
			bobCount++
		}
	}

	if bobCount != 1 {
		t.Errorf("Expected 'bob' to appear exactly once after dedup, got %d times in %v", bobCount, reviewers)
	}

	if len(reviewers) != 3 {
		t.Errorf("Expected 3 unique reviewers (alice, bob, carol), got %d: %v", len(reviewers), reviewers)
	}
}

func TestLookupTeamReviewers(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Test CLI flag takes precedence
	viper.Set(config.PrTeamReviewers, []string{"cli-team"})
	teamRevs := lookupTeamReviewers(ctx, "test-repo")
	if len(teamRevs) != 1 || teamRevs[0] != "cli-team" {
		t.Errorf("Expected [cli-team] from CLI flag, got %v", teamRevs)
	}

	// Test default team reviewers by repo name — all returned
	viper.Set(config.PrTeamReviewers, []string{})
	viper.Set(config.DefaultTeamReviewers, map[string][]string{
		"test-repo":  {"team-a", "team-b"},
		"other-repo": {"team-c"},
	})

	teamRevs = lookupTeamReviewers(ctx, "test-repo")
	if len(teamRevs) != 2 {
		t.Errorf("Expected 2 team reviewers, got %d: %v", len(teamRevs), teamRevs)
	}

	// Test label-based team reviewers
	catalog.Labels["platform"] = mapset.NewSet("test-repo")
	t.Cleanup(func() { delete(catalog.Labels, "platform") })

	viper.Set(config.DefaultTeamReviewers, map[string][]string{
		"~platform": {"infra-team"},
	})
	viper.Set(config.TokenLabel, "~")

	teamRevs = lookupTeamReviewers(ctx, "test-repo")
	if len(teamRevs) != 1 || teamRevs[0] != "infra-team" {
		t.Errorf("Expected [infra-team] from label, got %v", teamRevs)
	}
}
