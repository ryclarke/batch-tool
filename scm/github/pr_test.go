package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v74/github"

	"github.com/ryclarke/batch-tool/scm"
)

// newTestGithub creates a Github provider configured to use a test server
func newTestGithub(t *testing.T, server *httptest.Server) *Github {
	t.Helper()
	ctx := loadFixture(t)

	// Create a context with a test-appropriate timeout to prevent deadlocks
	// while still allowing cancellation to work
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	t.Cleanup(cancel)

	client := github.NewClient(http.DefaultClient)
	client.BaseURL, _ = client.BaseURL.Parse(server.URL + "/")

	return &Github{
		client:  client,
		project: "test-org",
		ctx:     ctx,
	}
}

// mockPRResponse creates a GitHub PR API response
func mockPRResponse(id int64, number int, title, body, branch string, mergeable bool, reviewers []string) map[string]interface{} {
	pr := map[string]interface{}{
		"id":              id,
		"number":          number,
		"title":           title,
		"body":            body,
		"mergeable":       mergeable,
		"mergeable_state": "clean",
		"head": map[string]interface{}{
			"ref": branch,
		},
		"base": map[string]interface{}{
			"ref": "main",
		},
	}

	if len(reviewers) > 0 {
		reviewerList := make([]map[string]interface{}, len(reviewers))
		for i, r := range reviewers {
			reviewerList[i] = map[string]interface{}{
				"login": r,
			}
		}
		pr["requested_reviewers"] = reviewerList
	}

	return pr
}

func TestGetPullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/repos/test-org/test-repo/pulls") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		// Return mock response
		prs := []map[string]interface{}{
			mockPRResponse(12345, 42, "Test PR", "PR description", "feature-branch", true, []string{"alice", "bob"}),
		}
		json.NewEncoder(w).Encode(prs)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.GetPullRequest("test-repo", "feature-branch")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.ID != 12345 {
		t.Errorf("Expected ID 12345, got %d", pr.ID)
	}
	if pr.Number != 42 {
		t.Errorf("Expected number 42, got %d", pr.Number)
	}
	if pr.Title != "Test PR" {
		t.Errorf("Expected title 'Test PR', got '%s'", pr.Title)
	}
	if pr.Description != "PR description" {
		t.Errorf("Expected description 'PR description', got '%s'", pr.Description)
	}
	if len(pr.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(pr.Reviewers))
	}
	if pr.Reviewers[0] != "alice" || pr.Reviewers[1] != "bob" {
		t.Errorf("Unexpected reviewers: %v", pr.Reviewers)
	}
}

func TestGetPullRequest_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return empty list (no PRs found)
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.GetPullRequest("test-repo", "nonexistent-branch")

	if err == nil {
		t.Fatal("Expected error for nonexistent PR")
	}
	if !strings.Contains(err.Error(), "no open pull request found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestGetPullRequest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal Server Error"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.GetPullRequest("test-repo", "feature-branch")

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestOpenPullRequest(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// First request: check for existing PR (return empty)
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pulls") {
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		// Second request: create PR
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/pulls") {
			// Verify request body
			var req github.NewPullRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if req.GetTitle() != "New Feature" {
				t.Errorf("Expected title 'New Feature', got '%s'", req.GetTitle())
			}

			// Return created PR
			pr := mockPRResponse(99999, 100, "New Feature", "Feature description", "feature-branch", true, nil)
			json.NewEncoder(w).Encode(pr)
			return
		}

		t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.OpenPullRequest("test-repo", "feature-branch", &scm.PROptions{
		Title:       "New Feature",
		Description: "Feature description",
		BaseBranch:  "main",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.Number != 100 {
		t.Errorf("Expected PR number 100, got %d", pr.Number)
	}
	if pr.Title != "New Feature" {
		t.Errorf("Expected title 'New Feature', got '%s'", pr.Title)
	}
}

func TestOpenPullRequest_AlreadyExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Return existing PR
		prs := []map[string]interface{}{
			mockPRResponse(12345, 42, "Existing PR", "description", "feature-branch", true, nil),
		}
		json.NewEncoder(w).Encode(prs)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.OpenPullRequest("test-repo", "feature-branch", &scm.PROptions{
		Title: "New PR",
	})

	if err == nil {
		t.Fatal("Expected error when PR already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestOpenPullRequest_WithReviewers(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Check existing PRs
		if requestPhase == 1 {
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		// Phase 2: Create PR
		if requestPhase == 2 && r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/pulls") {
			pr := mockPRResponse(99999, 100, "New Feature", "", "feature-branch", true, nil)
			json.NewEncoder(w).Encode(pr)
			return
		}

		// Phase 3: Request reviewers
		if requestPhase == 3 && r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/requested_reviewers") {
			// Verify reviewers in request
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Reviewers) != 2 {
				t.Errorf("Expected 2 reviewers, got %d", len(req.Reviewers))
			}

			pr := mockPRResponse(99999, 100, "New Feature", "", "feature-branch", true, req.Reviewers)
			json.NewEncoder(w).Encode(pr)
			return
		}

		t.Errorf("Unexpected request phase %d: %s %s", requestPhase, r.Method, r.URL.Path)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.OpenPullRequest("test-repo", "feature-branch", &scm.PROptions{
		Title:      "New Feature",
		BaseBranch: "main",
		Reviewers:  []string{"alice", "bob"},
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(pr.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(pr.Reviewers))
	}
}

func TestOpenPullRequest_NilOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check existing PRs
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}

		// Create PR - title should default to branch name
		if r.Method == http.MethodPost {
			var req github.NewPullRequest
			json.NewDecoder(r.Body).Decode(&req)

			// Title should default to branch name when nil options
			if req.GetTitle() != "feature-branch" {
				t.Errorf("Expected default title 'feature-branch', got '%s'", req.GetTitle())
			}

			pr := mockPRResponse(99999, 100, "feature-branch", "", "feature-branch", true, nil)
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.OpenPullRequest("test-repo", "feature-branch", nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.Title != "feature-branch" {
		t.Errorf("Expected title to default to branch name, got '%s'", pr.Title)
	}
}

func TestUpdatePullRequest(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Get existing PR
		if requestPhase == 1 && r.Method == http.MethodGet {
			prs := []map[string]interface{}{
				mockPRResponse(12345, 42, "Old Title", "Old description", "feature-branch", true, nil),
			}
			json.NewEncoder(w).Encode(prs)
			return
		}

		// Phase 2: Update PR
		if requestPhase == 2 && r.Method == http.MethodPatch {
			var req github.PullRequest
			json.NewDecoder(r.Body).Decode(&req)

			if req.GetTitle() != "Updated Title" {
				t.Errorf("Expected title 'Updated Title', got '%s'", req.GetTitle())
			}

			pr := mockPRResponse(12345, 42, "Updated Title", "Updated description", "feature-branch", true, nil)
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.UpdatePullRequest("test-repo", "feature-branch", &scm.PROptions{
		Title:       "Updated Title",
		Description: "Updated description",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", pr.Title)
	}
}

func TestUpdatePullRequest_WithReviewers(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Get existing PR
		if requestPhase == 1 {
			prs := []map[string]interface{}{
				mockPRResponse(12345, 42, "Title", "", "feature-branch", true, nil),
			}
			json.NewEncoder(w).Encode(prs)
			return
		}

		// Phase 2: Request reviewers
		if requestPhase == 2 && strings.Contains(r.URL.Path, "/requested_reviewers") {
			pr := mockPRResponse(12345, 42, "Title", "", "feature-branch", true, []string{"charlie"})
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.UpdatePullRequest("test-repo", "feature-branch", &scm.PROptions{
		Reviewers: []string{"charlie"},
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(pr.Reviewers) != 1 || pr.Reviewers[0] != "charlie" {
		t.Errorf("Expected reviewer 'charlie', got %v", pr.Reviewers)
	}
}

func TestUpdatePullRequest_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.UpdatePullRequest("test-repo", "nonexistent-branch", &scm.PROptions{
		Title: "Updated",
	})

	if err == nil {
		t.Fatal("Expected error for nonexistent PR")
	}
}

func TestMergePullRequest(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Get PR
		if requestPhase == 1 && r.Method == http.MethodGet {
			prs := []map[string]interface{}{
				mockPRResponse(12345, 42, "PR to Merge", "", "feature-branch", true, nil),
			}
			json.NewEncoder(w).Encode(prs)
			return
		}

		// Phase 2: Merge PR
		if requestPhase == 2 && r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/merge") {
			result := map[string]interface{}{
				"sha":     "abc123",
				"merged":  true,
				"message": "Pull Request successfully merged",
			}
			json.NewEncoder(w).Encode(result)
			return
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.MergePullRequest("test-repo", "feature-branch", false)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.Number != 42 {
		t.Errorf("Expected PR number 42, got %d", pr.Number)
	}
}

func TestMergePullRequest_NotMergeable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		prs := []map[string]interface{}{
			mockPRResponse(12345, 42, "PR", "", "feature-branch", false, nil),
		}
		json.NewEncoder(w).Encode(prs)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.MergePullRequest("test-repo", "feature-branch", false)

	if err == nil {
		t.Fatal("Expected error for non-mergeable PR")
	}
	if !strings.Contains(err.Error(), "not mergeable") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestMergePullRequest_ForceNonMergeable(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Get PR (not mergeable)
		if requestPhase == 1 {
			prs := []map[string]interface{}{
				mockPRResponse(12345, 42, "PR", "", "feature-branch", false, nil),
			}
			json.NewEncoder(w).Encode(prs)
			return
		}

		// Phase 2: Force merge anyway
		if requestPhase == 2 && strings.Contains(r.URL.Path, "/merge") {
			result := map[string]interface{}{
				"sha":     "abc123",
				"merged":  true,
				"message": "Pull Request successfully merged",
			}
			json.NewEncoder(w).Encode(result)
			return
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.MergePullRequest("test-repo", "feature-branch", true) // force=true

	if err != nil {
		t.Fatalf("Force merge should succeed: %v", err)
	}

	if pr.Number != 42 {
		t.Errorf("Expected PR number 42, got %d", pr.Number)
	}
}

func TestMergePullRequest_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.MergePullRequest("test-repo", "nonexistent-branch", false)

	if err == nil {
		t.Fatal("Expected error for nonexistent PR")
	}
}

func TestParsePR(t *testing.T) {
	tests := []struct {
		name     string
		input    *github.PullRequest
		expected *scm.PullRequest
	}{
		{
			name: "FullPR",
			input: &github.PullRequest{
				ID:     github.Ptr(int64(12345)),
				Number: github.Ptr(42),
				Title:  github.Ptr("Test PR"),
				Body:   github.Ptr("PR body"),
				RequestedReviewers: []*github.User{
					{Login: github.Ptr("alice")},
					{Login: github.Ptr("bob")},
				},
			},
			expected: &scm.PullRequest{
				ID:          12345,
				Number:      42,
				Title:       "Test PR",
				Description: "PR body",
				Reviewers:   []string{"alice", "bob"},
			},
		},
		{
			name: "MinimalPR",
			input: &github.PullRequest{
				ID:     github.Ptr(int64(1)),
				Number: github.Ptr(1),
			},
			expected: &scm.PullRequest{
				ID:        1,
				Number:    1,
				Reviewers: []string{},
			},
		},
		{
			name: "PRWithNilFields",
			input: &github.PullRequest{
				ID:                 github.Ptr(int64(100)),
				Number:             github.Ptr(50),
				Title:              nil,
				Body:               nil,
				RequestedReviewers: nil,
			},
			expected: &scm.PullRequest{
				ID:        100,
				Number:    50,
				Reviewers: []string{},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parsePR(tc.input)

			if result.ID != tc.expected.ID {
				t.Errorf("Expected ID %d, got %d", tc.expected.ID, result.ID)
			}
			if result.Number != tc.expected.Number {
				t.Errorf("Expected Number %d, got %d", tc.expected.Number, result.Number)
			}
			if result.Title != tc.expected.Title {
				t.Errorf("Expected Title '%s', got '%s'", tc.expected.Title, result.Title)
			}
			if result.Description != tc.expected.Description {
				t.Errorf("Expected Description '%s', got '%s'", tc.expected.Description, result.Description)
			}
			if len(result.Reviewers) != len(tc.expected.Reviewers) {
				t.Errorf("Expected %d reviewers, got %d", len(tc.expected.Reviewers), len(result.Reviewers))
			}
		})
	}
}

func TestUpdatePullRequest_ResetReviewers(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Get existing PR
		if requestPhase == 1 && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pulls") {
			prs := []map[string]interface{}{
				mockPRResponse(12345, 42, "Title", "", "feature-branch", true, []string{"alice", "bob"}),
			}
			json.NewEncoder(w).Encode(prs)
			return
		}

		// Phase 2: List current reviewers
		if requestPhase == 2 && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/requested_reviewers") {
			reviewers := map[string]interface{}{
				"users": []map[string]interface{}{
					{"login": "alice"},
					{"login": "bob"},
				},
			}
			json.NewEncoder(w).Encode(reviewers)
			return
		}

		// Phase 3: Remove old reviewers
		if requestPhase == 3 && r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/requested_reviewers") {
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			// Should remove alice (not in new list)
			if len(req.Reviewers) != 1 || req.Reviewers[0] != "alice" {
				t.Errorf("Expected to remove 'alice', got %v", req.Reviewers)
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Phase 4: Add new reviewers
		if requestPhase == 4 && r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/requested_reviewers") {
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			// Should add charlie (not in old list)
			if len(req.Reviewers) != 1 || req.Reviewers[0] != "charlie" {
				t.Errorf("Expected to add 'charlie', got %v", req.Reviewers)
			}

			pr := mockPRResponse(12345, 42, "Title", "", "feature-branch", true, []string{"bob", "charlie"})
			json.NewEncoder(w).Encode(pr)
			return
		}

		// Phase 5: Refresh PR by number to get updated reviewers
		if requestPhase == 5 && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/pulls/42") {
			pr := mockPRResponse(12345, 42, "Title", "", "feature-branch", true, []string{"bob", "charlie"})
			json.NewEncoder(w).Encode(pr)
			return
		}

		t.Errorf("Unexpected request phase %d: %s %s", requestPhase, r.Method, r.URL.Path)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.UpdatePullRequest("test-repo", "feature-branch", &scm.PROptions{
		Reviewers:      []string{"bob", "charlie"}, // Keep bob, remove alice, add charlie
		ResetReviewers: true,
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(pr.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(pr.Reviewers))
	}
}

func TestUpdatePullRequest_AppendReviewers(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Get existing PR with existing reviewers
		if requestPhase == 1 && r.Method == http.MethodGet {
			prs := []map[string]interface{}{
				mockPRResponse(12345, 42, "Title", "", "feature-branch", true, []string{"alice"}),
			}
			json.NewEncoder(w).Encode(prs)
			return
		}

		// Phase 2: Append reviewers (should NOT list/remove, just add)
		if requestPhase == 2 && r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/requested_reviewers") {
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Reviewers) != 1 || req.Reviewers[0] != "bob" {
				t.Errorf("Expected to append 'bob', got %v", req.Reviewers)
			}

			// Return PR with both reviewers
			pr := mockPRResponse(12345, 42, "Title", "", "feature-branch", true, []string{"alice", "bob"})
			json.NewEncoder(w).Encode(pr)
			return
		}

		t.Errorf("Unexpected request phase %d: %s %s", requestPhase, r.Method, r.URL.Path)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	pr, err := g.UpdatePullRequest("test-repo", "feature-branch", &scm.PROptions{
		Reviewers:      []string{"bob"}, // Append bob to existing alice
		ResetReviewers: false,           // Default behavior: append
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(pr.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers after append, got %d", len(pr.Reviewers))
	}
}

func TestReplaceReviewers_AllNew(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: List current reviewers
		if requestPhase == 1 && strings.Contains(r.URL.Path, "/requested_reviewers") {
			reviewers := map[string]interface{}{
				"users": []map[string]interface{}{
					{"login": "alice"},
					{"login": "bob"},
				},
			}
			json.NewEncoder(w).Encode(reviewers)
			return
		}

		// Phase 2: Remove all old reviewers
		if requestPhase == 2 && r.Method == http.MethodDelete {
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Reviewers) != 2 {
				t.Errorf("Expected to remove 2 reviewers, got %d", len(req.Reviewers))
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Phase 3: Add all new reviewers
		if requestPhase == 3 && r.Method == http.MethodPost {
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Reviewers) != 2 {
				t.Errorf("Expected to add 2 reviewers, got %d", len(req.Reviewers))
			}

			pr := mockPRResponse(12345, 42, "Title", "", "branch", true, req.Reviewers)
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)

	_, err := g.replaceReviewers("test-repo", 42, []string{"charlie", "david"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestReplaceReviewers_PartialOverlap(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: List current reviewers
		if requestPhase == 1 {
			reviewers := map[string]interface{}{
				"users": []map[string]interface{}{
					{"login": "alice"},
					{"login": "bob"},
				},
			}
			json.NewEncoder(w).Encode(reviewers)
			return
		}

		// Phase 2: Remove alice (not in new list)
		if requestPhase == 2 && r.Method == http.MethodDelete {
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Reviewers) != 1 || req.Reviewers[0] != "alice" {
				t.Errorf("Expected to remove 'alice', got %v", req.Reviewers)
			}

			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Phase 3: Add charlie (not in old list)
		if requestPhase == 3 && r.Method == http.MethodPost {
			var req github.ReviewersRequest
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Reviewers) != 1 || req.Reviewers[0] != "charlie" {
				t.Errorf("Expected to add 'charlie', got %v", req.Reviewers)
			}

			pr := mockPRResponse(12345, 42, "Title", "", "branch", true, []string{"bob", "charlie"})
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)

	// Replace alice,bob with bob,charlie (keep bob, remove alice, add charlie)
	_, err := g.replaceReviewers("test-repo", 42, []string{"bob", "charlie"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestReplaceReviewers_SameReviewers(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: List current reviewers
		if requestPhase == 1 {
			reviewers := map[string]interface{}{
				"users": []map[string]interface{}{
					{"login": "alice"},
					{"login": "bob"},
				},
			}
			json.NewEncoder(w).Encode(reviewers)
			return
		}

		// Phase 2: Get PR by number (no changes made, so just return current state)
		if requestPhase == 2 && strings.Contains(r.URL.Path, "/pulls/42") {
			pr := mockPRResponse(12345, 42, "Title", "", "branch", true, []string{"alice", "bob"})
			json.NewEncoder(w).Encode(pr)
			return
		}

		// Should not make any add/remove requests since reviewers are the same
		t.Errorf("Unexpected request phase %d: %s %s", requestPhase, r.Method, r.URL.Path)
	}))
	defer server.Close()

	g := newTestGithub(t, server)

	// Same reviewers - no changes needed
	_, err := g.replaceReviewers("test-repo", 42, []string{"alice", "bob"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if requestPhase != 2 {
		t.Errorf("Expected 2 requests (list + get), got %d", requestPhase)
	}
}

func TestListReviewers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/requested_reviewers") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		reviewers := map[string]interface{}{
			"users": []map[string]interface{}{
				{"login": "alice"},
				{"login": "bob"},
				{"login": "charlie"},
			},
		}
		json.NewEncoder(w).Encode(reviewers)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	reviewers, err := g.listReviewers("test-repo", 42)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(reviewers) != 3 {
		t.Errorf("Expected 3 reviewers, got %d", len(reviewers))
	}

	expected := []string{"alice", "bob", "charlie"}
	for i, r := range reviewers {
		if r != expected[i] {
			t.Errorf("Expected reviewer %s at index %d, got %s", expected[i], i, r)
		}
	}
}

func TestListReviewers_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reviewers := map[string]interface{}{
			"users": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(reviewers)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	reviewers, err := g.listReviewers("test-repo", 42)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(reviewers) != 0 {
		t.Errorf("Expected 0 reviewers, got %d", len(reviewers))
	}
}

func TestRemoveReviewers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Expected DELETE request, got %s", r.Method)
		}

		var req github.ReviewersRequest
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Reviewers) != 2 {
			t.Errorf("Expected 2 reviewers to remove, got %d", len(req.Reviewers))
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	err := g.removeReviewers("test-repo", 42, []string{"alice", "bob"})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestRemoveReviewers_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("Should not make any request when removing empty reviewer list")
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	err := g.removeReviewers("test-repo", 42, []string{})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestRemoveReviewers_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal error"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	err := g.removeReviewers("test-repo", 42, []string{"alice"})

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestListReviewers_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal error"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.listReviewers("test-repo", 42)

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestRequestReviewers_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"message": "Invalid reviewers"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.requestReviewers("test-repo", 42, []string{"invalid"})

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestEditPullRequest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "PR not found"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.editPullRequest("test-repo", 42, &github.PullRequest{Title: github.Ptr("Test")})

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestOpenPullRequest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]string{"message": "Validation failed"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	req := &github.NewPullRequest{
		Head:  github.Ptr("feature"),
		Base:  github.Ptr("main"),
		Title: github.Ptr("Test"),
	}
	_, err := g.openPullRequest("test-repo", req)

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestMergePullRequest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"message": "Cannot merge"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	err := g.mergePullRequest("test-repo", 42)

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

// TestTeamReviewersOpenPullRequest tests opening a PR with team reviewers
// Note: This test verifies the team reviewer flow is called without errors.
func TestTeamReviewersOpenPullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Create PR response
		if strings.HasSuffix(r.URL.Path, "/pulls") && r.Method == "POST" {
			pr := mockPRResponse(1, 1, "Test PR", "Test description", "feature/new", true, []string{})
			json.NewEncoder(w).Encode(pr)
		} else if strings.Contains(r.URL.Path, "/review_requests") && r.Method == "POST" {
			// Request reviewers response - return the PR with basic info
			pr := mockPRResponse(1, 1, "Test PR", "Test description", "feature/new", true, []string{})
			json.NewEncoder(w).Encode(pr)
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	opts := &scm.PROptions{
		Title:         "Test PR",
		Description:   "Test description",
		TeamReviewers: []string{"org/team1", "org/team2"},
		BaseBranch:    "main",
	}

	pr, err := g.OpenPullRequest("test-repo", "feature/new", opts)
	if err != nil {
		t.Fatalf("OpenPullRequest failed: %v", err)
	}
	if pr == nil {
		t.Fatal("expected PR, got nil")
	}
	// Verify PR was created successfully - team reviewers flow was executed
}

// TestTeamReviewersUpdatePullRequest tests updating a PR with team reviewers
func TestTeamReviewersUpdatePullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, "/pulls") && r.Method == "GET" {
			// List PRs response - return a PR
			prs := []map[string]interface{}{
				mockPRResponse(1, 1, "Test PR", "Test", "feature/update", true, []string{}),
			}
			json.NewEncoder(w).Encode(prs)
		} else if strings.Contains(r.URL.Path, "/pulls/1") && r.Method == "GET" {
			// Get specific PR
			pr := mockPRResponse(1, 1, "Test PR", "Test", "feature/update", true, []string{})
			json.NewEncoder(w).Encode(pr)
		} else if strings.Contains(r.URL.Path, "/review_requests") && r.Method == "POST" {
			// Request team reviewers response
			pr := mockPRResponse(1, 1, "Test PR", "Test", "feature/update", true, []string{})
			json.NewEncoder(w).Encode(pr)
		} else if strings.Contains(r.URL.Path, "/review_requests") && r.Method == "DELETE" {
			// Remove reviewers response
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	opts := &scm.PROptions{
		TeamReviewers:  []string{"org/team1"},
		ResetReviewers: false,
	}

	pr, err := g.UpdatePullRequest("test-repo", "feature/update", opts)
	if err != nil {
		t.Fatalf("UpdatePullRequest failed: %v", err)
	}
	if pr == nil {
		t.Fatal("expected PR, got nil")
	}
}

// TestTeamReviewersWithMixedReviewers tests adding both user and team reviewers
func TestTeamReviewersWithMixedReviewers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, "/pulls") && r.Method == "POST" {
			// Create PR response
			pr := mockPRResponse(1, 1, "Mixed PR", "Mixed reviewers", "feature/mixed", true, []string{})
			json.NewEncoder(w).Encode(pr)
		} else if strings.Contains(r.URL.Path, "/review_requests") && r.Method == "POST" {
			// Request reviewers response
			pr := mockPRResponse(1, 1, "Mixed PR", "Mixed reviewers", "feature/mixed", true, []string{})
			json.NewEncoder(w).Encode(pr)
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	opts := &scm.PROptions{
		Title:         "Mixed PR",
		Description:   "Mixed reviewers",
		Reviewers:     []string{"user1", "user2"},
		TeamReviewers: []string{"org/backend", "org/frontend"},
		BaseBranch:    "main",
	}

	pr, err := g.OpenPullRequest("test-repo", "feature/mixed", opts)
	if err != nil {
		t.Fatalf("OpenPullRequest failed: %v", err)
	}
	if pr == nil {
		t.Fatal("expected PR, got nil")
	}
}
