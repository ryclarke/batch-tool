package github

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestListTeamReviewers tests listing team reviewers with various scenarios
func TestListTeamReviewers(t *testing.T) {
	tests := []struct {
		name          string
		teams         []map[string]interface{}
		expectedCount int
		expectedFirst string
	}{
		{
			name: "multiple_teams",
			teams: []map[string]interface{}{
				{"name": "team1", "slug": "team1", "organization": map[string]interface{}{"login": "test-org"}},
				{"name": "team2", "slug": "team2", "organization": map[string]interface{}{"login": "test-org"}},
			},
			expectedCount: 2,
			expectedFirst: "test-org/team1",
		},
		{
			name:          "no_teams",
			teams:         []map[string]interface{}{},
			expectedCount: 0,
			expectedFirst: "",
		},
		{
			name: "single_team",
			teams: []map[string]interface{}{
				{"name": "backend", "slug": "backend", "organization": map[string]interface{}{"login": "myorg"}},
			},
			expectedCount: 1,
			expectedFirst: "myorg/backend",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"users": []map[string]interface{}{},
						"teams": tt.teams,
					})
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			g := newTestGithub(t, server)

			teams, err := g.listTeamReviewers("test-repo", 42)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(teams) != tt.expectedCount {
				t.Errorf("Expected %d teams, got %d", tt.expectedCount, len(teams))
			}
			if tt.expectedCount > 0 && teams[0] != tt.expectedFirst {
				t.Errorf("Expected first team '%s', got %q", tt.expectedFirst, teams[0])
			}
		})
	}
}

// TestRemoveTeamReviewers tests removing team reviewers with various scenarios
func TestRemoveTeamReviewers(t *testing.T) {
	tests := []struct {
		name              string
		reviewersToRemove []string
		expectRequest     bool
		expectedCount     int
	}{
		{
			name:              "single_team",
			reviewersToRemove: []string{"org/team-to-remove"},
			expectRequest:     true,
			expectedCount:     1,
		},
		{
			name:              "multiple_teams",
			reviewersToRemove: []string{"org/team1", "org/team2"},
			expectRequest:     true,
			expectedCount:     2,
		},
		{
			name:              "empty_list",
			reviewersToRemove: []string{},
			expectRequest:     false,
			expectedCount:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestMade := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodDelete && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
					requestMade = true
					var req map[string]interface{}
					json.NewDecoder(r.Body).Decode(&req)

					if teams, ok := req["team_reviewers"]; !ok {
						t.Error("Expected team_reviewers in DELETE request")
					} else if teamList, ok := teams.([]interface{}); !ok || len(teamList) != tt.expectedCount {
						t.Errorf("Expected %d team reviewers in request, got: %v", tt.expectedCount, teams)
					}
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			g := newTestGithub(t, server)

			err := g.removeTeamReviewers("test-repo", 42, tt.reviewersToRemove)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if requestMade != tt.expectRequest {
				t.Errorf("Expected request to be made: %v, but was: %v", tt.expectRequest, requestMade)
			}
		})
	}
}

// TestReplaceTeamReviewers tests replacing team reviewers with various scenarios
func TestReplaceTeamReviewers(t *testing.T) {
	tests := []struct {
		name                string
		currentTeams        []map[string]interface{}
		teamsToRequest      []string
		expectPostRequest   bool
		expectDeleteRequest bool
	}{
		{
			name:                "all_new",
			currentTeams:        []map[string]interface{}{},
			teamsToRequest:      []string{"org/new-team"},
			expectPostRequest:   true,
			expectDeleteRequest: false,
		},
		{
			name: "replace_all",
			currentTeams: []map[string]interface{}{
				{"name": "old-team", "slug": "old-team", "organization": map[string]interface{}{"login": "test-org"}},
			},
			teamsToRequest:      []string{"org/new-team"},
			expectPostRequest:   true,
			expectDeleteRequest: true,
		},
		{
			name:                "multiple_new",
			currentTeams:        []map[string]interface{}{},
			teamsToRequest:      []string{"org/team1", "org/team2"},
			expectPostRequest:   true,
			expectDeleteRequest: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			postRequestMade := false
			deleteRequestMade := false

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
					// Return current team reviewers
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]interface{}{
						"users": []map[string]interface{}{},
						"teams": tt.currentTeams,
					})
					return
				}

				if r.Method == http.MethodPost && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
					// Check if team_reviewers are in request
					postRequestMade = true
					var req map[string]interface{}
					json.NewDecoder(r.Body).Decode(&req)
					if _, hasTeam := req["team_reviewers"]; !hasTeam {
						t.Errorf("Expected team_reviewers in request, got: %v", req)
					}
					w.WriteHeader(http.StatusOK)
					pr := mockPRResponse(1, 42, "Test PR", "Test", "feature/test", true, []string{})
					pr["requested_reviewers"] = []map[string]interface{}{}
					pr["requested_teams"] = []map[string]interface{}{
						{"name": "new-team", "slug": "new-team"},
					}
					json.NewEncoder(w).Encode(pr)
					return
				}

				if r.Method == http.MethodDelete && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
					deleteRequestMade = true
					w.WriteHeader(http.StatusOK)
					return
				}

				if r.Method == http.MethodGet && r.URL.Path == "/repos/test-org/test-repo/pulls/42" {
					w.Header().Set("Content-Type", "application/json")
					pr := mockPRResponse(1, 42, "Test PR", "Test", "feature/test", true, []string{})
					pr["requested_teams"] = []map[string]interface{}{
						{"name": "new-team", "slug": "new-team"},
					}
					json.NewEncoder(w).Encode(pr)
					return
				}

				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			g := newTestGithub(t, server)

			_, err := g.replaceTeamReviewers("test-repo", 42, tt.teamsToRequest)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if postRequestMade != tt.expectPostRequest {
				t.Errorf("Expected POST request: %v, but was: %v", tt.expectPostRequest, postRequestMade)
			}
			if deleteRequestMade != tt.expectDeleteRequest {
				t.Errorf("Expected DELETE request: %v, but was: %v", tt.expectDeleteRequest, deleteRequestMade)
			}
		})
	}
}

// TestRequestTeamReviewers tests requesting team reviewers with error handling
func TestRequestTeamReviewers(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(t *testing.T) *httptest.Server
		wantErr   bool
		errMsg    string
	}{
		{
			name: "success",
			setupMock: func(_ *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
						w.WriteHeader(http.StatusOK)
						json.NewEncoder(w).Encode(map[string]interface{}{
							"id":     1,
							"number": 42,
						})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr: false,
		},
		{
			name: "api_error",
			setupMock: func(_ *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
						w.WriteHeader(http.StatusInternalServerError)
						json.NewEncoder(w).Encode(map[string]interface{}{
							"message": "Internal Server Error",
						})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr: true,
			errMsg:  "failed to request team reviewers",
		},
		{
			name: "empty_teams",
			setupMock: func(_ *testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == http.MethodPost && r.URL.Path == "/repos/test-org/test-repo/pulls/42/requested_reviewers" {
						w.WriteHeader(http.StatusOK)
						json.NewEncoder(w).Encode(map[string]interface{}{})
						return
					}
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupMock(t)
			defer server.Close()

			g := newTestGithub(t, server)

			_, err := g.requestTeamReviewers("test-repo", 42, []string{"org/team1"})
			if (err != nil) != tt.wantErr {
				t.Errorf("requestTeamReviewers() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
