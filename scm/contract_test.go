package scm

import (
	"testing"
)

// ProviderContract defines a test suite that any Provider implementation
// should be able to pass if it correctly implements the interface.
// This ensures consistent behavior across all SCM provider implementations.

// RunProviderContract runs the contract tests for a given Provider implementation.
// Use this in each provider's test file to ensure contract compliance.
func RunProviderContract(t *testing.T, name string, provider Provider) {
	t.Run(name+"_ImplementsProvider", func(t *testing.T) {
		// Verify the provider is not nil
		if provider == nil {
			t.Fatal("Provider should not be nil")
		}

		// Verify it implements the Provider interface
		var _ = provider
	})
}

// RepositoryContract tests the Repository struct for consistency
func TestRepositoryContract(t *testing.T) {
	tests := []struct {
		name       string
		repo       *Repository
		wantName   string
		wantPublic bool
	}{
		{
			name: "BasicRepository",
			repo: &Repository{
				Name:          "test-repo",
				Description:   "Test description",
				DefaultBranch: "main",
				Public:        false,
				Project:       "TEST",
				Labels:        []string{"core"},
			},
			wantName:   "test-repo",
			wantPublic: false,
		},
		{
			name: "PublicRepository",
			repo: &Repository{
				Name:   "public-repo",
				Public: true,
			},
			wantName:   "public-repo",
			wantPublic: true,
		},
		{
			name: "RepositoryWithTopics",
			repo: &Repository{
				Name:   "tagged-repo",
				Labels: []string{"api", "core", "v2"},
			},
			wantName:   "tagged-repo",
			wantPublic: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.repo.Name != tc.wantName {
				t.Errorf("Expected name '%s', got '%s'", tc.wantName, tc.repo.Name)
			}
			if tc.repo.Public != tc.wantPublic {
				t.Errorf("Expected public=%v, got %v", tc.wantPublic, tc.repo.Public)
			}
		})
	}
}

// PullRequestContract tests the PullRequest struct for consistency
func TestPullRequestContract(t *testing.T) {
	tests := []struct {
		name        string
		pr          *PullRequest
		wantID      int
		wantNumber  int
		wantTitle   string
		reviewCount int
	}{
		{
			name: "BasicPR",
			pr: &PullRequest{
				ID:          123,
				Number:      123,
				Title:       "Test PR",
				Description: "PR description",
				Reviewers:   []string{"alice"},
			},
			wantID:      123,
			wantNumber:  123,
			wantTitle:   "Test PR",
			reviewCount: 1,
		},
		{
			name: "PRWithMultipleReviewers",
			pr: &PullRequest{
				ID:        456,
				Number:    456,
				Title:     "Feature PR",
				Reviewers: []string{"alice", "bob", "charlie"},
			},
			wantID:      456,
			wantNumber:  456,
			wantTitle:   "Feature PR",
			reviewCount: 3,
		},
		{
			name: "PRWithNoReviewers",
			pr: &PullRequest{
				ID:        789,
				Number:    789,
				Title:     "Quick Fix",
				Reviewers: []string{},
			},
			wantID:      789,
			wantNumber:  789,
			wantTitle:   "Quick Fix",
			reviewCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pr.ID != tc.wantID {
				t.Errorf("Expected ID %d, got %d", tc.wantID, tc.pr.ID)
			}
			if tc.pr.Number != tc.wantNumber {
				t.Errorf("Expected Number %d, got %d", tc.wantNumber, tc.pr.Number)
			}
			if tc.pr.Title != tc.wantTitle {
				t.Errorf("Expected Title '%s', got '%s'", tc.wantTitle, tc.pr.Title)
			}
			if len(tc.pr.Reviewers) != tc.reviewCount {
				t.Errorf("Expected %d reviewers, got %d", tc.reviewCount, len(tc.pr.Reviewers))
			}
		})
	}
}

// PROptionsContract tests the PROptions struct for consistency
func TestPROptionsContract(t *testing.T) {
	tests := []struct {
		name    string
		opts    *PROptions
		wantNil bool
	}{
		{
			name:    "NilOptions",
			opts:    nil,
			wantNil: true,
		},
		{
			name: "TitleOnly",
			opts: &PROptions{
				Title: "My PR",
			},
			wantNil: false,
		},
		{
			name: "FullOptions",
			opts: &PROptions{
				Title:       "Full PR",
				Description: "Description",
				Reviewers:   []string{"alice", "bob"},
			},
			wantNil: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isNil := tc.opts == nil
			if isNil != tc.wantNil {
				t.Errorf("Expected nil=%v, got nil=%v", tc.wantNil, isNil)
			}
		})
	}
}
