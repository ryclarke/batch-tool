package github

import (
	"context"
	"testing"

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
	shouldRetry, retErr := g.handleRateLimitError(context.TODO(), err, false)

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
