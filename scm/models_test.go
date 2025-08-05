package scm

import (
	"testing"
)

func TestPullRequestMethods(t *testing.T) {
	// Create a test pull request
	pr := PullRequest{
		"id":      float64(123),
		"version": float64(2),
		"title":   "Test PR",
		"state":   "OPEN",
		"reviewers": []any{
			map[string]any{
				"user": map[string]any{
					"name": "reviewer1",
				},
			},
			map[string]any{
				"user": map[string]any{
					"name": "reviewer2",
				},
			},
		},
	}

	// Test ID method
	if pr.ID() != 123 {
		t.Errorf("Expected ID 123, got %d", pr.ID())
	}

	// Test Version method
	if pr.Version() != 2 {
		t.Errorf("Expected version 2, got %d", pr.Version())
	}

	// Test GetReviewers method
	reviewers := pr.GetReviewers()
	if len(reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(reviewers))
	}
	if reviewers[0] != "reviewer1" {
		t.Errorf("Expected first reviewer to be 'reviewer1', got %s", reviewers[0])
	}
	if reviewers[1] != "reviewer2" {
		t.Errorf("Expected second reviewer to be 'reviewer2', got %s", reviewers[1])
	}
}

func TestPullRequestIDMissing(t *testing.T) {
	pr := PullRequest{}
	if pr.ID() != 0 {
		t.Errorf("Expected ID 0 for empty PR, got %d", pr.ID())
	}
}

func TestPullRequestVersionMissing(t *testing.T) {
	pr := PullRequest{}
	if pr.Version() != 0 {
		t.Errorf("Expected version 0 for empty PR, got %d", pr.Version())
	}
}

func TestPullRequestAddReviewers(t *testing.T) {
	pr := PullRequest{
		"reviewers": []any{},
	}

	pr.AddReviewers([]string{"alice", "bob"})

	reviewers := pr.GetReviewers()
	if len(reviewers) != 2 {
		t.Errorf("Expected 2 reviewers after adding, got %d", len(reviewers))
	}

	if reviewers[0] != "alice" {
		t.Errorf("Expected first reviewer to be 'alice', got %s", reviewers[0])
	}
	if reviewers[1] != "bob" {
		t.Errorf("Expected second reviewer to be 'bob', got %s", reviewers[1])
	}
}

func TestPullRequestSetReviewers(t *testing.T) {
	pr := PullRequest{
		"reviewers": []any{
			map[string]any{
				"user": map[string]any{
					"name": "old-reviewer",
				},
			},
		},
	}

	pr.SetReviewers([]string{"new-reviewer1", "new-reviewer2"})

	reviewers := pr.GetReviewers()
	if len(reviewers) != 2 {
		t.Errorf("Expected 2 reviewers after setting, got %d", len(reviewers))
	}

	if reviewers[0] != "new-reviewer1" {
		t.Errorf("Expected first reviewer to be 'new-reviewer1', got %s", reviewers[0])
	}
	if reviewers[1] != "new-reviewer2" {
		t.Errorf("Expected second reviewer to be 'new-reviewer2', got %s", reviewers[1])
	}
}

func TestPullRequestAppendReviewers(t *testing.T) {
	pr := PullRequest{
		"reviewers": []any{
			map[string]any{
				"user": map[string]any{
					"name": "existing-reviewer",
				},
			},
		},
	}

	pr.AddReviewers([]string{"new-reviewer"})

	reviewers := pr.GetReviewers()
	if len(reviewers) != 2 {
		t.Errorf("Expected 2 reviewers after adding, got %d", len(reviewers))
	}

	if reviewers[0] != "existing-reviewer" {
		t.Errorf("Expected first reviewer to be 'existing-reviewer', got %s", reviewers[0])
	}
	if reviewers[1] != "new-reviewer" {
		t.Errorf("Expected second reviewer to be 'new-reviewer', got %s", reviewers[1])
	}
}

func TestRepositoryStruct(t *testing.T) {
	repo := Repository{
		Name:          "test-repo",
		Description:   "A test repository",
		Public:        true,
		Project:       "test-project",
		DefaultBranch: "main",
		Labels:        []string{"backend", "go", "testing"},
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected name 'test-repo', got %s", repo.Name)
	}

	if repo.Description != "A test repository" {
		t.Errorf("Expected description 'A test repository', got %s", repo.Description)
	}

	if !repo.Public {
		t.Error("Expected repository to be public")
	}

	if repo.Project != "test-project" {
		t.Errorf("Expected project 'test-project', got %s", repo.Project)
	}

	if repo.DefaultBranch != "main" {
		t.Errorf("Expected default branch 'main', got %s", repo.DefaultBranch)
	}

	if len(repo.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(repo.Labels))
	}

	expectedLabels := []string{"backend", "go", "testing"}
	for i, label := range repo.Labels {
		if label != expectedLabels[i] {
			t.Errorf("Expected label %s at position %d, got %s", expectedLabels[i], i, label)
		}
	}
}
