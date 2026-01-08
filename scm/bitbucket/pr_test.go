package bitbucket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/scm"
)

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

// newTestBitbucket creates a Bitbucket provider configured to use a test server
func newTestBitbucket(t *testing.T, server *httptest.Server) *Bitbucket {
	t.Helper()
	ctx := loadFixture(t)

	// Parse the server URL to get scheme and host:port
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %v", err)
	}

	return &Bitbucket{
		client:  server.Client(),
		scheme:  serverURL.Scheme,
		host:    serverURL.Host,
		project: "TEST",
		ctx:     ctx,
	}
}

// mockBitbucketPRResponse creates a Bitbucket PR API response
func mockBitbucketPRResponse(id float64, title, description, branch string, reviewers []string) map[string]interface{} {
	pr := map[string]interface{}{
		"id":          id,
		"version":     float64(1),
		"title":       title,
		"description": description,
		"fromRef": map[string]interface{}{
			"id": "refs/heads/" + branch,
			"repository": map[string]interface{}{
				"slug": "test-repo",
				"project": map[string]interface{}{
					"key": "TEST",
				},
			},
		},
		"toRef": map[string]interface{}{
			"id": "refs/heads/main",
			"repository": map[string]interface{}{
				"slug": "test-repo",
				"project": map[string]interface{}{
					"key": "TEST",
				},
			},
		},
	}

	if len(reviewers) > 0 {
		reviewerList := make([]map[string]interface{}, len(reviewers))
		for i, r := range reviewers {
			reviewerList[i] = map[string]interface{}{
				"user": map[string]interface{}{
					"name": r,
				},
			}
		}
		pr["reviewers"] = reviewerList
	}

	return pr
}

func TestGetPullRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/pull-requests") {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}

		// Verify query params
		if r.URL.Query().Get("direction") != "outgoing" {
			t.Errorf("Expected direction=outgoing, got %s", r.URL.Query().Get("direction"))
		}

		// Return mock response
		resp := map[string]interface{}{
			"values": []map[string]interface{}{
				mockBitbucketPRResponse(42, "Test PR", "PR description", "feature-branch", []string{"alice", "bob"}),
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	pr, err := b.GetPullRequest("test-repo", "feature-branch")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.ID != 42 {
		t.Errorf("Expected ID 42, got %d", pr.ID)
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
}

func TestGetPullRequest_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return empty list
		resp := map[string]interface{}{
			"values": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	_, err := b.GetPullRequest("test-repo", "nonexistent-branch")

	if err == nil {
		t.Fatal("Expected error for nonexistent PR")
	}
	if !strings.Contains(err.Error(), "no pull requests found") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestGetPullRequest_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors": [{"message": "Internal Server Error"}]}`))
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	_, err := b.GetPullRequest("test-repo", "feature-branch")

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestOpenPullRequest(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Check if PR exists (should return empty)
		if requestPhase == 1 && r.Method == http.MethodGet {
			resp := map[string]interface{}{
				"values": []map[string]interface{}{},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Phase 2: Create PR
		if requestPhase == 2 && r.Method == http.MethodPost {
			// Verify request body
			var req prResp
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
			}

			if req.Title != "New Feature" {
				t.Errorf("Expected title 'New Feature', got '%s'", req.Title)
			}

			// Return created PR
			pr := mockBitbucketPRResponse(100, "New Feature", "Feature description", "feature-branch", nil)
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	pr, err := b.OpenPullRequest("test-repo", "feature-branch", &scm.PROptions{
		Title:       "New Feature",
		Description: "Feature description",
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.Title != "New Feature" {
		t.Errorf("Expected title 'New Feature', got '%s'", pr.Title)
	}
}

func TestOpenPullRequest_WithReviewers(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Check if PR exists (should return empty)
		if requestPhase == 1 && r.Method == http.MethodGet {
			resp := map[string]interface{}{
				"values": []map[string]interface{}{},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Phase 2: Create PR with reviewers
		if requestPhase == 2 && r.Method == http.MethodPost {
			var req prResp
			json.NewDecoder(r.Body).Decode(&req)

			if len(req.Reviewers) != 2 {
				t.Errorf("Expected 2 reviewers in request, got %d", len(req.Reviewers))
			}

			pr := mockBitbucketPRResponse(100, "New Feature", "", "feature-branch", []string{"alice", "bob"})
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	pr, err := b.OpenPullRequest("test-repo", "feature-branch", &scm.PROptions{
		Title:     "New Feature",
		Reviewers: []string{"alice", "bob"},
	})

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(pr.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(pr.Reviewers))
	}
}

func TestOpenPullRequest_NilOptions(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Check if PR exists (should return empty)
		if requestPhase == 1 && r.Method == http.MethodGet {
			resp := map[string]interface{}{
				"values": []map[string]interface{}{},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Phase 2: Create PR
		if requestPhase == 2 && r.Method == http.MethodPost {
			var req prResp
			json.NewDecoder(r.Body).Decode(&req)

			// Title should default to branch name
			if req.Title != "feature-branch" {
				t.Errorf("Expected default title 'feature-branch', got '%s'", req.Title)
			}

			pr := mockBitbucketPRResponse(100, "feature-branch", "", "feature-branch", nil)
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	pr, err := b.OpenPullRequest("test-repo", "feature-branch", nil)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.Title != "feature-branch" {
		t.Errorf("Expected title to default to branch name, got '%s'", pr.Title)
	}
}

func TestOpenPullRequest_AlreadyExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return existing PR
		resp := map[string]interface{}{
			"values": []map[string]interface{}{
				mockBitbucketPRResponse(42, "Existing PR", "", "feature-branch", nil),
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	_, err := b.OpenPullRequest("test-repo", "feature-branch", &scm.PROptions{
		Title: "New Feature",
	})

	if err == nil {
		t.Fatal("Expected error for existing PR")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}
}

func TestUpdatePullRequest(t *testing.T) {
	requestPhase := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPhase++

		// Phase 1: Get existing PR
		if requestPhase == 1 && r.Method == http.MethodGet {
			resp := map[string]interface{}{
				"values": []map[string]interface{}{
					mockBitbucketPRResponse(42, "Old Title", "Old description", "feature-branch", nil),
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Phase 2: Update PR
		if requestPhase == 2 && r.Method == http.MethodPut {
			var req prResp
			json.NewDecoder(r.Body).Decode(&req)

			if req.Title != "Updated Title" {
				t.Errorf("Expected title 'Updated Title', got '%s'", req.Title)
			}

			pr := mockBitbucketPRResponse(42, "Updated Title", "Updated description", "feature-branch", nil)
			json.NewEncoder(w).Encode(pr)
			return
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	pr, err := b.UpdatePullRequest("test-repo", "feature-branch", &scm.PROptions{
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

func TestUpdatePullRequest_NoChanges(t *testing.T) {
	b := &Bitbucket{
		project: "TEST",
	}

	_, err := b.UpdatePullRequest("test-repo", "feature-branch", &scm.PROptions{})

	if err == nil {
		t.Fatal("Expected error for no updates")
	}
	if !strings.Contains(err.Error(), "no updates provided") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestUpdatePullRequest_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"values": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	_, err := b.UpdatePullRequest("test-repo", "nonexistent-branch", &scm.PROptions{
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
			resp := map[string]interface{}{
				"values": []map[string]interface{}{
					mockBitbucketPRResponse(42, "PR to Merge", "", "feature-branch", nil),
				},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Phase 2: Merge PR
		if requestPhase == 2 && r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/merge") {
			// Verify version param
			if r.URL.Query().Get("version") == "" {
				t.Error("Expected version query parameter")
			}

			result := map[string]interface{}{
				"state": "MERGED",
			}
			json.NewEncoder(w).Encode(result)
			return
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	pr, err := b.MergePullRequest("test-repo", "feature-branch", false)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if pr.ID != 42 {
		t.Errorf("Expected PR ID 42, got %d", pr.ID)
	}
}

func TestMergePullRequest_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"values": []map[string]interface{}{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	_, err := b.MergePullRequest("test-repo", "nonexistent-branch", false)

	if err == nil {
		t.Fatal("Expected error for nonexistent PR")
	}
}

func TestParsePR(t *testing.T) {
	tests := []struct {
		name     string
		input    *prResp
		expected *scm.PullRequest
	}{
		{
			name: "FullPR",
			input: &prResp{
				ID:          float64(123),
				Version:     float64(5),
				Title:       "Test PR",
				Description: "PR body",
				Reviewers: []prRev{
					{User: prRevUser{Name: "alice"}},
					{User: prRevUser{Name: "bob"}},
				},
			},
			expected: &scm.PullRequest{
				ID:          123,
				Number:      123,
				Title:       "Test PR",
				Description: "PR body",
				Reviewers:   []string{"alice", "bob"},
			},
		},
		{
			name: "MinimalPR",
			input: &prResp{
				ID: float64(1),
			},
			expected: &scm.PullRequest{
				ID:        1,
				Number:    1,
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

func TestGenPR(t *testing.T) {
	ctx := loadFixture(t)
	b := &Bitbucket{
		project: "TEST",
		ctx:     ctx,
	}

	payload := b.genPR("test-repo", "feature-branch", "main", "Test Title", "Test Description", []string{"alice", "bob"})

	if payload == "" {
		t.Fatal("Expected non-empty payload")
	}

	// Verify it's valid JSON
	var pr prResp
	if err := json.Unmarshal([]byte(payload), &pr); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if pr.Title != "Test Title" {
		t.Errorf("Expected title 'Test Title', got '%s'", pr.Title)
	}
	if pr.Description != "Test Description" {
		t.Errorf("Expected description 'Test Description', got '%s'", pr.Description)
	}
	if len(pr.Reviewers) != 2 {
		t.Errorf("Expected 2 reviewers, got %d", len(pr.Reviewers))
	}
	if pr.FromRef.ID != "refs/heads/feature-branch" {
		t.Errorf("Expected fromRef 'refs/heads/feature-branch', got '%s'", pr.FromRef.ID)
	}
	if pr.ToRef.ID != "refs/heads/main" {
		t.Errorf("Expected toRef 'refs/heads/main', got '%s'", pr.ToRef.ID)
	}
}

func TestGenPR_DefaultBaseBranch(t *testing.T) {
	ctx := loadFixture(t)
	b := &Bitbucket{
		project: "TEST",
		ctx:     ctx,
	}

	// Empty baseBranch should use configured default
	payload := b.genPR("test-repo", "feature-branch", "", "Test Title", "", nil)

	var pr prResp
	if err := json.Unmarshal([]byte(payload), &pr); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	// Should fall back to configured default branch (from fixture)
	if pr.ToRef.ID == "" {
		t.Error("Expected toRef to have default branch")
	}
}
