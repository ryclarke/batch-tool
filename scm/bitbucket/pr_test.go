package bitbucket

import "testing"

func TestPullRequestMethods(t *testing.T) {
	// Create a test pull request
	pr := &prResp{
		ID:      float64(123),
		Version: float64(2),
		Title:   "Test PR",
		Reviewers: []prRev{
			{
				User: prRevUser{Name: "reviewer1"},
			},
			{
				User: prRevUser{Name: "reviewer2"},
			},
		},
	}

	// Test ID method
	if pr.ID != 123 {
		t.Errorf("Expected ID 123, got %f", pr.ID)
	}

	// Test Version method
	if pr.Version != 2 {
		t.Errorf("Expected version 2, got %f", pr.Version)
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
	pr := &prResp{}
	if pr.ID != 0 {
		t.Errorf("Expected ID 0 for empty PR, got %f", pr.ID)
	}
}

func TestPullRequestVersionMissing(t *testing.T) {
	pr := &prResp{}
	if pr.Version != 0 {
		t.Errorf("Expected version 0 for empty PR, got %f", pr.Version)
	}
}

func TestPullRequestAddReviewers(t *testing.T) {
	pr := &prResp{
		Reviewers: []prRev{},
	}

	pr.AddReviewers([]string{"alice", "bob"})

	reviewers := pr.GetReviewers()
	if len(reviewers) != 2 {
		t.Fatalf("Expected 2 reviewers after adding, got %d", len(reviewers))
	}

	if reviewers[0] != "alice" {
		t.Fatalf("Expected first reviewer to be 'alice', got %s", reviewers[0])
	}

	if reviewers[1] != "bob" {
		t.Fatalf("Expected second reviewer to be 'bob', got %s", reviewers[1])
	}
}

func TestPullRequestSetReviewers(t *testing.T) {
	pr := &prResp{
		Reviewers: []prRev{
			{
				User: prRevUser{Name: "old-reviewer"},
			},
		},
	}

	pr.SetReviewers([]string{"new-reviewer1", "new-reviewer2"})

	reviewers := pr.GetReviewers()
	if len(reviewers) != 2 {
		t.Fatalf("Expected 2 reviewers after setting, got %d", len(reviewers))
	}

	if reviewers[0] != "new-reviewer1" {
		t.Fatalf("Expected first reviewer to be 'new-reviewer1', got %s", reviewers[0])
	}
	if reviewers[1] != "new-reviewer2" {
		t.Fatalf("Expected second reviewer to be 'new-reviewer2', got %s", reviewers[1])
	}
}

func TestPullRequestAppendReviewers(t *testing.T) {
	pr := &prResp{
		Reviewers: []prRev{
			{
				User: prRevUser{Name: "existing-reviewer"},
			},
		},
	}

	pr.AddReviewers([]string{"new-reviewer"})

	reviewers := pr.GetReviewers()
	if len(reviewers) != 2 {
		t.Fatalf("Expected 2 reviewers after adding, got %d", len(reviewers))
	}

	if reviewers[0] != "existing-reviewer" {
		t.Fatalf("Expected first reviewer to be 'existing-reviewer', got %s", reviewers[0])
	}
	if reviewers[1] != "new-reviewer" {
		t.Fatalf("Expected second reviewer to be 'new-reviewer', got %s", reviewers[1])
	}
}
