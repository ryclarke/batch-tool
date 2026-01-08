package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockRepoResponse creates a GitHub repository API response
func mockRepoResponse(name, description string, private bool, defaultBranch string, topics []string) map[string]interface{} {
	return map[string]interface{}{
		"name":           name,
		"description":    description,
		"private":        private,
		"default_branch": defaultBranch,
		"topics":         topics,
	}
}

func TestListRepositories_Organization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify it's requesting org repos
		if !strings.Contains(r.URL.Path, "/orgs/test-org/repos") {
			t.Errorf("Expected org repos path, got %s", r.URL.Path)
		}

		// Return repos without pagination for simplicity
		repos := []map[string]interface{}{
			mockRepoResponse("repo-1", "First repo", false, "main", []string{"go", "cli"}),
			mockRepoResponse("repo-2", "Second repo", true, "master", []string{"python"}),
			mockRepoResponse("repo-3", "Third repo", false, "develop", nil),
		}
		json.NewEncoder(w).Encode(repos)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	repos, err := g.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 3 {
		t.Fatalf("Expected 3 repositories, got %d", len(repos))
	}

	// Verify first repo
	if repos[0].Name != "repo-1" {
		t.Errorf("Expected name 'repo-1', got '%s'", repos[0].Name)
	}
	if repos[0].Description != "First repo" {
		t.Errorf("Expected description 'First repo', got '%s'", repos[0].Description)
	}
	if repos[0].Public != true {
		t.Errorf("Expected public=true, got %v", repos[0].Public)
	}
	if repos[0].DefaultBranch != "main" {
		t.Errorf("Expected default branch 'main', got '%s'", repos[0].DefaultBranch)
	}
	if len(repos[0].Labels) != 2 || repos[0].Labels[0] != "go" {
		t.Errorf("Expected labels [go, cli], got %v", repos[0].Labels)
	}

	// Verify second repo (private)
	if repos[1].Public != false {
		t.Errorf("Expected public=false for private repo, got %v", repos[1].Public)
	}

	// Verify third repo
	if repos[2].DefaultBranch != "develop" {
		t.Errorf("Expected default branch 'develop', got '%s'", repos[2].DefaultBranch)
	}
}

func TestListRepositories_UserFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First request to org returns 404
		if strings.Contains(r.URL.Path, "/orgs/") {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
			return
		}

		// Fallback to user repos
		if strings.Contains(r.URL.Path, "/users/test-org/repos") {
			repos := []map[string]interface{}{
				mockRepoResponse("user-repo", "User repo", false, "main", nil),
			}
			json.NewEncoder(w).Encode(repos)
			return
		}

		t.Errorf("Unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	repos, err := g.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(repos))
	}

	if repos[0].Name != "user-repo" {
		t.Errorf("Expected name 'user-repo', got '%s'", repos[0].Name)
	}
}

func TestListRepositories_EmptyDefaultBranch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repos := []map[string]interface{}{
			{
				"name":           "empty-branch-repo",
				"description":    "Repo without default branch set",
				"private":        false,
				"default_branch": "", // Empty default branch
				"topics":         nil,
			},
		}
		json.NewEncoder(w).Encode(repos)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	repos, err := g.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should fall back to configured default branch
	if repos[0].DefaultBranch == "" {
		t.Error("Expected default branch to be set from config fallback")
	}
}

func TestListRepositories_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"message": "Internal Server Error"})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	_, err := g.ListRepositories()

	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestListRepositories_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	repos, err := g.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 0 {
		t.Errorf("Expected 0 repositories, got %d", len(repos))
	}
}

func TestListRepositories_ProjectAssignment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repos := []map[string]interface{}{
			mockRepoResponse("test-repo", "", false, "main", nil),
		}
		json.NewEncoder(w).Encode(repos)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	repos, err := g.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Project should be set to the provider's project
	if repos[0].Project != "test-org" {
		t.Errorf("Expected project 'test-org', got '%s'", repos[0].Project)
	}
}

func TestListRepositories_Pagination(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		page := r.URL.Query().Get("page")
		if page == "" || page == "0" {
			page = "1"
		}

		switch page {
		case "1":
			repos := []map[string]interface{}{
				mockRepoResponse("repo-page1", "", false, "main", nil),
			}
			w.Header().Set("Link", `<`+r.URL.Host+r.URL.Path+`?page=2>; rel="next"`)
			json.NewEncoder(w).Encode(repos)
		case "2":
			repos := []map[string]interface{}{
				mockRepoResponse("repo-page2", "", false, "main", nil),
			}
			w.Header().Set("Link", `<`+r.URL.Host+r.URL.Path+`?page=3>; rel="next"`)
			json.NewEncoder(w).Encode(repos)
		case "3":
			repos := []map[string]interface{}{
				mockRepoResponse("repo-page3", "", false, "main", nil),
			}
			// No "next" link - last page
			json.NewEncoder(w).Encode(repos)
		default:
			json.NewEncoder(w).Encode([]map[string]interface{}{})
		}
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	repos, err := g.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos) != 3 {
		t.Errorf("Expected 3 repositories across pages, got %d", len(repos))
	}
}

func TestListRepositories_TopicsMapping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		repos := []map[string]interface{}{
			mockRepoResponse("labeled-repo", "", false, "main", []string{"backend", "api", "golang"}),
		}
		json.NewEncoder(w).Encode(repos)
	}))
	defer server.Close()

	g := newTestGithub(t, server)
	repos, err := g.ListRepositories()

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(repos[0].Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(repos[0].Labels))
	}

	expectedLabels := []string{"backend", "api", "golang"}
	for i, label := range repos[0].Labels {
		if label != expectedLabels[i] {
			t.Errorf("Expected label '%s', got '%s'", expectedLabels[i], label)
		}
	}
}
