package bitbucket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockBitbucketRepoResponse creates a mock repository response
func mockBitbucketRepoResponse(name, description string, public bool, projectKey string) map[string]interface{} {
	return map[string]interface{}{
		"slug":        name,
		"name":        name,
		"description": description,
		"public":      public,
		"project": map[string]interface{}{
			"key":  projectKey,
			"name": projectKey + " Project",
		},
		"links": map[string]interface{}{
			"clone": []map[string]interface{}{
				{
					"href": "ssh://git@bitbucket.example.com/" + strings.ToLower(projectKey) + "/" + name + ".git",
					"name": "ssh",
				},
			},
		},
	}
}

// mockLabelsResponse creates a mock labels response
func mockLabelsResponse(labels ...string) map[string]interface{} {
	values := make([]map[string]interface{}, len(labels))
	for i, label := range labels {
		values[i] = map[string]interface{}{"name": label}
	}
	return map[string]interface{}{
		"values":     values,
		"isLastPage": true,
	}
}

// mockDefaultBranchResponse creates a mock default branch response
func mockDefaultBranchResponse(branchName string) map[string]interface{} {
	return map[string]interface{}{
		"displayId": branchName,
		"id":        "refs/heads/" + branchName,
		"isDefault": true,
	}
}

func TestListRepositories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route based on path
		switch {
		case strings.HasSuffix(r.URL.Path, "/repos"):
			// Repository list request
			resp := map[string]interface{}{
				"values": []map[string]interface{}{
					mockBitbucketRepoResponse("repo-one", "First repository", false, "TEST"),
					mockBitbucketRepoResponse("repo-two", "Second repository", true, "TEST"),
				},
				"isLastPage": true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/labels"):
			// Labels request
			repoName := extractRepoName(r.URL.Path)
			if repoName == "repo-one" {
				json.NewEncoder(w).Encode(mockLabelsResponse("core", "api"))
			} else {
				json.NewEncoder(w).Encode(mockLabelsResponse("frontend"))
			}
		case strings.HasSuffix(r.URL.Path, "/default-branch"):
			// Default branch request
			json.NewEncoder(w).Encode(mockDefaultBranchResponse("main"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	repos, err := b.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("Expected 2 repositories, got %d", len(repos))
	}

	// Check first repo
	if repos[0].Name != "repo-one" {
		t.Errorf("Expected repo name 'repo-one', got '%s'", repos[0].Name)
	}
	if repos[0].Description != "First repository" {
		t.Errorf("Expected description 'First repository', got '%s'", repos[0].Description)
	}
	if repos[0].DefaultBranch != "main" {
		t.Errorf("Expected default branch 'main', got '%s'", repos[0].DefaultBranch)
	}
	if len(repos[0].Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(repos[0].Labels))
	}

	// Check second repo
	if repos[1].Name != "repo-two" {
		t.Errorf("Expected repo name 'repo-two', got '%s'", repos[1].Name)
	}
	if len(repos[1].Labels) != 1 {
		t.Errorf("Expected 1 label, got %d", len(repos[1].Labels))
	}
}

func TestListRepositories_LargeLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/repos"):
			// Verify limit parameter is set to 1000
			limit := r.URL.Query().Get("limit")
			if limit != "1000" {
				t.Errorf("Expected limit=1000, got limit=%s", limit)
			}
			resp := map[string]interface{}{
				"values": []map[string]interface{}{
					mockBitbucketRepoResponse("repo-one", "", false, "TEST"),
					mockBitbucketRepoResponse("repo-two", "", false, "TEST"),
				},
				"isLastPage": true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/labels"):
			json.NewEncoder(w).Encode(mockLabelsResponse())
		case strings.HasSuffix(r.URL.Path, "/default-branch"):
			json.NewEncoder(w).Encode(mockDefaultBranchResponse("main"))
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	repos, err := b.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 2 {
		t.Fatalf("Expected 2 repositories, got %d", len(repos))
	}
}

func TestListRepositories_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/repos") {
			resp := map[string]interface{}{
				"values":     []map[string]interface{}{},
				"isLastPage": true,
			}
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	repos, err := b.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 0 {
		t.Errorf("Expected 0 repositories, got %d", len(repos))
	}
}

func TestListRepositories_LabelsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/repos"):
			resp := map[string]interface{}{
				"values": []map[string]interface{}{
					mockBitbucketRepoResponse("repo-one", "", false, "TEST"),
				},
				"isLastPage": true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/labels"):
			// Labels endpoint fails - the actual implementation returns an error
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"errors": [{"message": "Internal Error"}]}`))
		case strings.HasSuffix(r.URL.Path, "/default-branch"):
			json.NewEncoder(w).Encode(mockDefaultBranchResponse("main"))
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	_, err := b.ListRepositories()

	// The actual implementation returns an error when labels fail
	if err == nil {
		t.Fatal("Expected error when labels endpoint fails")
	}
}

func TestListRepositories_DefaultBranchFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/repos"):
			resp := map[string]interface{}{
				"values": []map[string]interface{}{
					mockBitbucketRepoResponse("repo-one", "", false, "TEST"),
				},
				"isLastPage": true,
			}
			json.NewEncoder(w).Encode(resp)
		case strings.HasSuffix(r.URL.Path, "/labels"):
			json.NewEncoder(w).Encode(mockLabelsResponse("core"))
		case strings.HasSuffix(r.URL.Path, "/default-branch"):
			// Default branch endpoint fails
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"errors": [{"message": "Not Found"}]}`))
		}
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	repos, err := b.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should still return the repo, just without default branch
	if len(repos) != 1 {
		t.Fatalf("Expected 1 repository, got %d", len(repos))
	}
}

func TestListRepositories_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errors": [{"message": "Internal Server Error"}]}`))
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	_, err := b.ListRepositories()

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestGetLabels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		json.NewEncoder(w).Encode(mockLabelsResponse("label1", "label2", "label3"))
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	labels, err := b.getLabels("test-repo")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(labels) != 3 {
		t.Fatalf("Expected 3 labels, got %d", len(labels))
	}

	expected := []string{"label1", "label2", "label3"}
	for i, label := range labels {
		if label != expected[i] {
			t.Errorf("Expected label '%s', got '%s'", expected[i], label)
		}
	}
}

func TestGetLabels_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockLabelsResponse())
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	labels, err := b.getLabels("test-repo")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(labels) != 0 {
		t.Errorf("Expected 0 labels, got %d", len(labels))
	}
}

func TestGetDefaultBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		json.NewEncoder(w).Encode(mockDefaultBranchResponse("develop"))
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	branch, err := b.getDefaultBranch("test-repo")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if branch != "develop" {
		t.Errorf("Expected branch 'develop', got '%s'", branch)
	}
}

func TestGetDefaultBranch_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	b := newTestBitbucket(t, server)
	branch, err := b.getDefaultBranch("test-repo")

	// Should return empty string and error on failure
	if err == nil {
		t.Error("Expected error on 404 response")
	}
	if branch != "" {
		t.Errorf("Expected empty branch on error, got '%s'", branch)
	}
}

// extractRepoName extracts the repository name from a URL path like /rest/api/1.0/projects/TEST/repos/repo-name/labels
func extractRepoName(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "repos" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
