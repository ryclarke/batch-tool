package fake_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ryclarke/batch-tool/scm"
	. "github.com/ryclarke/batch-tool/scm/fake"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../../config")
}

func TestNew(t *testing.T) {
	ctx := loadFixture(t)
	provider := New(ctx, "test-project")
	f, ok := provider.(*Fake)
	if !ok {
		t.Fatal("Expected provider to be of type *Fake")
	}

	if f.Project != "test-project" {
		t.Errorf("Expected project to be 'test-project', got %s", f.Project)
	}

	if len(f.Repositories) != 0 {
		t.Errorf("Expected 0 repositories, got %d", len(f.Repositories))
	}

	if len(f.PullRequests) != 0 {
		t.Errorf("Expected 0 pull requests, got %d", len(f.PullRequests))
	}
}

func TestNewWithData(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	if f.Project != "test-project" {
		t.Errorf("Expected project to be 'test-project', got %s", f.Project)
	}

	if len(f.Repositories) != len(testRepos) {
		t.Errorf("Expected %d repositories, got %d", len(testRepos), len(f.Repositories))
	}

	// Verify repositories were deep copied
	repos, err := f.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	for i, repo := range repos {
		if repo == testRepos[i] {
			t.Error("Repository should be a copy, not the same pointer")
		}

		if repo.Name != testRepos[i].Name {
			t.Errorf("Repository name mismatch: expected %s, got %s", testRepos[i].Name, repo.Name)
		}
	}
}

func TestListRepositories(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	repos, err := f.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	if len(repos) != 5 {
		t.Errorf("Expected 5 repositories, got %d", len(repos))
	}

	// Verify specific repository details
	repo1 := repos[0]
	if repo1.Name != "repo-1" {
		t.Errorf("Expected first repository to be 'repo-1', got %s", repo1.Name)
	}

	if len(repo1.Labels) != 3 {
		t.Errorf("Expected repo-1 to have 3 labels, got %d", len(repo1.Labels))
	}
}

func TestListRepositoriesWithError(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	expectedErr := fmt.Errorf("test error")
	f.SetError("ListRepositories", expectedErr)

	repos, err := f.ListRepositories()
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if repos != nil {
		t.Error("Expected nil repositories when error occurs")
	}
}

func TestOpenPullRequest(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	reviewers := []string{"reviewer1", "reviewer2"}
	pr, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: reviewers})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	if pr.ID != 1 {
		t.Errorf("Expected PR ID to be 1, got %d", pr.ID)
	}

	if pr.Version != 1 {
		t.Errorf("Expected PR version to be 1, got %d", pr.Version)
	}

	if pr.Title != "Test PR" {
		t.Errorf("Expected PR title to be 'Test PR', got %v", pr.Title)
	}

	if len(pr.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(pr.Reviewers))
	}

	if !f.HasPullRequest("repo-1", "feature-branch") {
		t.Error("Expected pull request to exist for repo-1:feature-branch")
	}
}

func TestOpenPullRequestDuplicate(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Open first PR
	_, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	// Try to open duplicate PR
	_, err = f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR 2", Description: "Test description 2", Reviewers: []string{}})
	if err == nil {
		t.Error("Expected error when opening duplicate pull request")
	}
}

func TestGetPullRequest(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Open a PR first
	originalPR, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{"reviewer1"}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	// Get the PR
	retrievedPR, err := f.GetPullRequest("repo-1", "feature-branch")
	if err != nil {
		t.Fatalf("Failed to get pull request: %v", err)
	}

	if retrievedPR.ID != originalPR.ID {
		t.Errorf("Expected PR ID %d, got %d", originalPR.ID, retrievedPR.ID)
	}

	if retrievedPR.Title != originalPR.Title {
		t.Errorf("Expected PR title %v, got %v", originalPR.Title, retrievedPR.Title)
	}
}

func TestGetPullRequestNotFound(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	_, err := f.GetPullRequest("repo-1", "nonexistent-branch")
	if err == nil {
		t.Error("Expected error when getting nonexistent pull request")
	}
}

func TestUpdatePullRequest(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Open a PR first
	_, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{"reviewer1"}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	// Update the PR
	updatedPR, err := f.UpdatePullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Updated PR", Description: "Updated description", Reviewers: []string{"reviewer2"}})
	if err != nil {
		t.Fatalf("Failed to update pull request: %v", err)
	}

	if updatedPR.Title != "Updated PR" {
		t.Errorf("Expected updated title 'Updated PR', got %v", updatedPR.Title)
	}

	if updatedPR.Version != 2 {
		t.Errorf("Expected PR version to be 2 after update, got %d", updatedPR.Version)
	}

	reviewers := updatedPR.Reviewers
	if len(reviewers) != 1 || reviewers[0] != "reviewer2" {
		t.Errorf("Expected reviewers to be [reviewer2], got %v", reviewers)
	}
}

func TestUpdatePullRequestAppendReviewers(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Open a PR first
	_, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{"reviewer1"}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	// Update the PR with append reviewers
	updatedPR, err := f.UpdatePullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Updated PR", Description: "Updated description", Reviewers: []string{"reviewer2", "reviewer1"}, AppendReviewers: true})
	if err != nil {
		t.Fatalf("Failed to update pull request: %v", err)
	}

	if len(updatedPR.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers after append, got %d", len(updatedPR.Reviewers))
	}

	// Should have both reviewers but no duplicates
	expectedReviewers := map[string]bool{"reviewer1": true, "reviewer2": true}
	for _, reviewer := range updatedPR.Reviewers {
		if !expectedReviewers[reviewer] {
			t.Errorf("Unexpected reviewer %s", reviewer)
		}
	}
}

func TestMergePullRequest(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Open a PR first
	_, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{"reviewer1"}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	// Merge the PR
	mergedPR, err := f.MergePullRequest("repo-1", "feature-branch", false)
	if err != nil {
		t.Fatalf("Failed to merge pull request: %v", err)
	}

	if _, ok := f.PullRequests[mergedPR.Repo]; ok {
		t.Error("Expected PR to be merged, but it is still open")
	}
}

func TestMergePullRequestAlreadyMerged(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Open and merge a PR
	_, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{"reviewer1"}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	_, err = f.MergePullRequest("repo-1", "feature-branch", false)
	if err != nil {
		t.Fatalf("Failed to merge pull request: %v", err)
	}

	// Try to merge again
	_, err = f.MergePullRequest("repo-1", "feature-branch", false)
	if err == nil {
		t.Error("Expected error when merging already merged pull request")
	}
}

// TestMergePullRequestMergeability tests mergeability checking with force flag
func TestMergePullRequestMergeability(t *testing.T) {
	tests := []struct {
		name        string
		mergeable   bool
		force       bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "mergeable PR without force",
			mergeable:   true,
			force:       false,
			expectError: false,
		},
		{
			name:        "mergeable PR with force",
			mergeable:   true,
			force:       true,
			expectError: false,
		},
		{
			name:        "unmergeable PR without force",
			mergeable:   false,
			force:       false,
			expectError: true,
			errorMsg:    "is not mergeable",
		},
		{
			name:        "unmergeable PR with force (bypass check)",
			mergeable:   false,
			force:       true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testRepos := CreateTestRepositories("test-project")
			f := NewFake("test-project", testRepos)

			// Open a PR
			pr, err := f.OpenPullRequest("repo-1", "test-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{"reviewer1"}})
			if err != nil {
				t.Fatalf("Failed to open pull request: %v", err)
			}

			// Set mergeability
			err = f.SetPRMergeable("repo-1", "test-branch", tt.mergeable)
			if err != nil {
				t.Fatalf("Failed to set PR mergeable status: %v", err)
			}

			// Verify the mergeable status was set
			updatedPR, err := f.GetPullRequest("repo-1", "test-branch")
			if err != nil {
				t.Fatalf("Failed to get pull request: %v", err)
			}
			if updatedPR.Mergeable != tt.mergeable {
				t.Errorf("Expected mergeable=%v, got %v", tt.mergeable, updatedPR.Mergeable)
			}

			// Attempt to merge
			mergedPR, err := f.MergePullRequest("repo-1", "test-branch", tt.force)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
				}
				// PR should still exist when merge fails
				_, stillExists := f.PullRequests["repo-1:test-branch"]
				if !stillExists {
					t.Error("Expected PR to still exist after failed merge")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if mergedPR == nil {
					t.Error("Expected merged PR to be returned")
				} else if mergedPR.ID != pr.ID {
					t.Errorf("Expected merged PR ID %d, got %d", pr.ID, mergedPR.ID)
				}
				// PR should be deleted after successful merge
				_, stillExists := f.PullRequests["repo-1:test-branch"]
				if stillExists {
					t.Error("Expected PR to be deleted after successful merge")
				}
			}
		})
	}
}

// TestMergePullRequestForceBypassesChecks tests that force flag bypasses false negatives
func TestMergePullRequestForceBypassesChecks(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Scenario: Provider incorrectly reports PR as unmergeable (false negative)
	// but we know it's actually fine and want to bypass the check
	_, err := f.OpenPullRequest("repo-1", "hotfix-branch", &scm.PROptions{Title: "Critical Fix", Description: "Emergency fix", Reviewers: []string{"reviewer1"}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	// Simulate false negative - provider says it's not mergeable
	err = f.SetPRMergeable("repo-1", "hotfix-branch", false)
	if err != nil {
		t.Fatalf("Failed to set PR mergeable status: %v", err)
	}

	// Without force, merge should fail
	_, err = f.MergePullRequest("repo-1", "hotfix-branch", false)
	if err == nil {
		t.Error("Expected merge to fail without force flag")
	}

	// PR should still exist
	pr, err := f.GetPullRequest("repo-1", "hotfix-branch")
	if err != nil {
		t.Fatalf("Expected PR to still exist: %v", err)
	}

	// With force, merge should succeed despite unmergeable status
	mergedPR, err := f.MergePullRequest("repo-1", "hotfix-branch", true)
	if err != nil {
		t.Errorf("Expected force merge to succeed: %v", err)
	}

	if mergedPR.ID != pr.ID {
		t.Errorf("Expected merged PR ID %d, got %d", pr.ID, mergedPR.ID)
	}

	// PR should be deleted after forced merge
	_, err = f.GetPullRequest("repo-1", "hotfix-branch")
	if err == nil {
		t.Error("Expected PR to be deleted after forced merge")
	}
}

// TestSetPRMergeableNotFound tests error handling when PR doesn't exist
func TestSetPRMergeableNotFound(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	err := f.SetPRMergeable("nonexistent-repo", "nonexistent-branch", true)
	if err == nil {
		t.Error("Expected error when setting mergeable status for nonexistent PR")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestAddRepository(t *testing.T) {
	ctx := loadFixture(t)
	fake := New(ctx, "test-project").(*Fake)

	repo := &scm.Repository{
		Name:          "new-repo",
		Description:   "New test repository",
		Public:        true,
		Project:       "test-project",
		DefaultBranch: "main",
		Labels:        []string{"test", "new"},
	}

	fake.AddRepository(repo)

	if fake.GetRepositoryCount() != 1 {
		t.Errorf("Expected 1 repository after adding, got %d", fake.GetRepositoryCount())
	}

	repos, err := fake.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to list repositories: %v", err)
	}

	if repos[0].Name != "new-repo" {
		t.Errorf("Expected repository name 'new-repo', got %s", repos[0].Name)
	}

	// Verify it's a copy, not the original
	if repos[0] == repo {
		t.Error("Repository should be a copy, not the same pointer")
	}
}

func TestGetRepositoryByName(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	repo := f.GetRepositoryByName("repo-1")
	if repo == nil {
		t.Fatal("Expected to find repo-1")
	}

	if repo.Name != "repo-1" {
		t.Errorf("Expected repository name 'repo-1', got %s", repo.Name)
	}

	nonExistent := f.GetRepositoryByName("nonexistent")
	if nonExistent != nil {
		t.Error("Expected nil for nonexistent repository")
	}
}

func TestGetRepositoriesByLabel(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	activeRepos := f.GetRepositoriesByLabel("active")
	if len(activeRepos) != 3 {
		t.Errorf("Expected 3 active repositories, got %d", len(activeRepos))
	}

	backendRepos := f.GetRepositoriesByLabel("backend")
	if len(backendRepos) != 2 {
		t.Errorf("Expected 2 backend repositories, got %d", len(backendRepos))
	}

	nonExistentRepos := f.GetRepositoriesByLabel("nonexistent")
	if len(nonExistentRepos) != 0 {
		t.Errorf("Expected 0 repositories for nonexistent label, got %d", len(nonExistentRepos))
	}
}

func TestGetAllLabels(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	labels := f.GetAllLabels()
	expectedLabels := []string{"active", "backend", "deprecated", "experimental", "frontend", "go", "javascript", "legacy", "microservice", "poc", "python"}

	if len(labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
	}

	for i, label := range labels {
		if label != expectedLabels[i] {
			t.Errorf("Expected label %s at position %d, got %s", expectedLabels[i], i, label)
		}
	}
}

func TestErrorHandling(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	tests := []struct {
		method   string
		testFunc func() error
	}{
		{
			"GetPullRequest",
			func() error {
				_, err := f.GetPullRequest("repo-1", "branch-1")
				return err
			},
		},
		{
			"OpenPullRequest",
			func() error {
				_, err := f.OpenPullRequest("repo-1", "branch-1", &scm.PROptions{Title: "title", Description: "desc", Reviewers: []string{}})
				return err
			},
		},
		{
			"UpdatePullRequest",
			func() error {
				_, err := f.UpdatePullRequest("repo-1", "branch-1", &scm.PROptions{Title: "title", Description: "desc", Reviewers: []string{}})
				return err
			},
		},
		{
			"MergePullRequest",
			func() error {
				_, err := f.MergePullRequest("repo-1", "branch-1", false)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			expectedErr := fmt.Errorf("test error for %s", test.method)
			f.SetError(test.method, expectedErr)

			err := test.testFunc()
			if !errors.Is(err, expectedErr) {
				t.Errorf("Expected error %v, got %v", expectedErr, err)
			}

			f.ClearError(test.method)

			// Error should be cleared now
			err = test.testFunc()
			// Some methods will still fail (like getting nonexistent PR), but not with our configured error
			if errors.Is(err, expectedErr) {
				t.Error("Expected error to be cleared")
			}
		})
	}
}

func TestClear(t *testing.T) {
	testRepos := CreateTestRepositories("test-project")
	f := NewFake("test-project", testRepos)

	// Add a pull request
	_, err := f.OpenPullRequest("repo-1", "feature-branch", &scm.PROptions{Title: "Test PR", Description: "Test description", Reviewers: []string{}})
	if err != nil {
		t.Fatalf("Failed to open pull request: %v", err)
	}

	if f.GetRepositoryCount() == 0 {
		t.Fatal("Expected repositories before clear")
	}

	if f.GetPullRequestCount() == 0 {
		t.Fatal("Expected pull requests before clear")
	}

	f.Clear()

	if f.GetRepositoryCount() != 0 {
		t.Errorf("Expected 0 repositories after clear, got %d", f.GetRepositoryCount())
	}

	if f.GetPullRequestCount() != 0 {
		t.Errorf("Expected 0 pull requests after clear, got %d", f.GetPullRequestCount())
	}
}

func TestCreateTestRepositories(t *testing.T) {
	repos := CreateTestRepositories("test-project")

	if len(repos) != 5 {
		t.Errorf("Expected 5 test repositories, got %d", len(repos))
	}

	for i, repo := range repos {
		expectedName := fmt.Sprintf("repo-%d", i+1)
		if repo.Name != expectedName {
			t.Errorf("Expected repository name %s, got %s", expectedName, repo.Name)
		}

		if repo.Project != "test-project" {
			t.Errorf("Expected project 'test-project', got %s", repo.Project)
		}

		if len(repo.Labels) == 0 {
			t.Errorf("Expected repository %s to have labels", repo.Name)
		}
	}
}
