package github

import (
	"context"
	"testing"

	"github.com/ryclarke/batch-tool/scm"
	testhelper "github.com/ryclarke/batch-tool/utils/test"
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
	var _ scm.Provider = provider
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

func TestGithubProviderMethods(t *testing.T) {
	ctx := loadFixture(t)
	provider := New(ctx, "test-project")

	// Test that all interface methods exist
	// Note: These will likely fail without proper authentication/network setup
	// but we can at least verify the methods exist and handle errors gracefully

	_, err := provider.ListRepositories()
	if err == nil {
		t.Log("ListRepositories succeeded (unexpected without auth)")
	} else {
		t.Logf("ListRepositories failed as expected: %v", err)
	}

	_, err = provider.GetPullRequest("test-repo", "test-branch")
	if err == nil {
		t.Log("GetPullRequest succeeded (unexpected without auth)")
	} else {
		t.Logf("GetPullRequest failed as expected: %v", err)
	}

	_, err = provider.OpenPullRequest("test-repo", "test-branch", "Test PR", "Description", []string{})
	if err == nil {
		t.Log("OpenPullRequest succeeded (unexpected without auth)")
	} else {
		t.Logf("OpenPullRequest failed as expected: %v", err)
	}

	_, err = provider.UpdatePullRequest("test-repo", "test-branch", "Updated", "Updated desc", []string{}, false)
	if err == nil {
		t.Log("UpdatePullRequest succeeded (unexpected without auth)")
	} else {
		t.Logf("UpdatePullRequest failed as expected: %v", err)
	}

	_, err = provider.MergePullRequest("test-repo", "test-branch", false)
	if err == nil {
		t.Log("MergePullRequest succeeded (unexpected without auth)")
	} else {
		t.Logf("MergePullRequest failed as expected: %v", err)
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
