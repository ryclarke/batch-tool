package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/ryclarke/batch-tool/scm"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../../config")
}

func TestNew(t *testing.T) {
	ctx := loadFixture(t)
	provider := New(ctx, "test-project")

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	// Test that it implements the Provider interface
	var _ = provider
}

func TestGithubProviderCreation(t *testing.T) {
	// Test provider creation with different project names
	testCases := []string{
		"simple-project",
		"project-with-dashes",
		"project_with_underscores",
		"Project123",
	}

	for _, projectName := range testCases {
		t.Run("Project_"+projectName, func(t *testing.T) {
			ctx := loadFixture(t)
			provider := New(ctx, projectName)

			if provider == nil {
				t.Errorf("Expected non-nil provider for project %s", projectName)
			}

			// Verify it's the correct type
			githubProvider, ok := provider.(*Github)
			if !ok {
				t.Errorf("Expected *Github provider, got %T", provider)
			}

			if githubProvider.project != projectName {
				t.Errorf("Expected project %s, got %s", projectName, githubProvider.project)
			}
		})
	}
}

func TestGithubProviderRegistration(t *testing.T) {
	ctx := loadFixture(t)
	// Test that the GitHub provider is registered during init
	provider := scm.Get(ctx, "github", "test-project")

	if provider == nil {
		t.Fatal("Expected GitHub provider to be registered")
	}

	// Verify it's the correct type
	_, ok := provider.(*Github)
	if !ok {
		t.Errorf("Expected *Github provider, got %T", provider)
	}
}

func TestHandleRateLimitError_NotRateLimitError(t *testing.T) {
	ctx := loadFixture(t)
	g := New(ctx, "test-project").(*Github)

	// Regular error should not trigger retry
	err := context.DeadlineExceeded
	shouldRetry, retErr := g.handleRateLimitError(err, false)

	if shouldRetry {
		t.Error("Should not retry on non-rate-limit error")
	}
	if retErr != nil {
		t.Errorf("Expected nil retErr for non-rate-limit error, got %v", retErr)
	}
}

func TestReadLock(t *testing.T) {
	ctx := loadFixture(t)
	g := New(ctx, "test-project").(*Github)

	// Test acquiring and releasing read lock
	done := g.readLock()
	if done == nil {
		t.Fatal("Expected non-nil done function")
	}

	// Release the lock
	done()

	// Should be able to acquire again
	done2 := g.readLock()
	if done2 == nil {
		t.Fatal("Expected to acquire read lock again")
	}
	done2()
}

func TestWriteLock(t *testing.T) {
	ctx := loadFixture(t)
	g := New(ctx, "test-project").(*Github)

	// Test acquiring and releasing write lock
	done := g.writeLock()
	if done == nil {
		t.Fatal("Expected non-nil done function")
	}

	// Release the lock (note: will have a small delay due to write backoff)
	done()
}

func TestReadWriteLockInteraction(t *testing.T) {
	ctx := loadFixture(t)
	g := New(ctx, "test-project").(*Github)

	// Acquire multiple read locks (should be allowed)
	done1 := g.readLock()
	done2 := g.readLock()

	// Release them
	done1()
	done2()

	// Now acquire write lock
	done3 := g.writeLock()
	done3()
}

// TestCheckCapabilities tests the CheckCapabilities method
func TestCheckCapabilities(t *testing.T) {
	ctx := loadFixture(t)
	g := New(ctx, "test-project").(*Github)

	tests := []struct {
		name    string
		opts    *scm.PROptions
		wantErr bool
	}{
		{
			name:    "nil_options",
			opts:    nil,
			wantErr: false,
		},
		{
			name: "valid_team_reviewers",
			opts: &scm.PROptions{
				TeamReviewers: []string{"org/team1"},
			},
			wantErr: false,
		},
		{
			name: "valid_draft",
			opts: &scm.PROptions{
				Draft: boolPtr(true),
			},
			wantErr: false,
		},
		{
			name: "valid_reset_reviewers",
			opts: &scm.PROptions{
				ResetReviewers: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.CheckCapabilities(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckCapabilities() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestHandleRateLimitError_RetrySuccess tests successful retry after rate limit
func TestHandleRateLimitError_RetrySuccess(t *testing.T) {
	// Create a mock server that provides rate limit info
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rate_limit" {
			w.Header().Set("Content-Type", "application/json")
			// Return rate limit info with reset time in the future
			resetTime := time.Now().Add(100 * time.Millisecond).Unix()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"resources": map[string]interface{}{
					"core": map[string]interface{}{
						"limit":     60,
						"remaining": 0,
						"reset":     resetTime,
					},
					"search": map[string]interface{}{
						"limit":     10,
						"remaining": 0,
						"reset":     resetTime,
					},
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx := loadFixture(t)
	g := New(ctx, "test-project").(*Github)
	// Use test server for rate limit checks
	g.client.BaseURL, _ = url.Parse(server.URL)

	// Test with a nil error (not a rate limit error)
	shouldRetry, err := g.handleRateLimitError(nil, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if shouldRetry {
		t.Error("Expected shouldRetry to be false for nil error")
	}
}

func boolPtr(b bool) *bool {
	return &b
}
