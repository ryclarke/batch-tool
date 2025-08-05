package bitbucket

import (
	"testing"

	"github.com/ryclarke/batch-tool/scm"
)

func TestNew(t *testing.T) {
	provider := New("test-project")

	if provider == nil {
		t.Fatal("Expected non-nil provider")
	}

	// Test that it implements the Provider interface
	var _ scm.Provider = provider
}

func TestBitbucketProviderCreation(t *testing.T) {
	// Test provider creation with different project names
	testCases := []string{
		"simple-project",
		"project-with-dashes",
		"project_with_underscores",
		"Project123",
	}

	for _, projectName := range testCases {
		t.Run("Project_"+projectName, func(t *testing.T) {
			provider := New(projectName)

			if provider == nil {
				t.Errorf("Expected non-nil provider for project %s", projectName)
			}

			// Verify it's the correct type
			bitbucketProvider, ok := provider.(*Bitbucket)
			if !ok {
				t.Errorf("Expected *Bitbucket provider, got %T", provider)
			}

			if bitbucketProvider.project != projectName {
				t.Errorf("Expected project %s, got %s", projectName, bitbucketProvider.project)
			}
		})
	}
}

func TestBitbucketProviderInterface(t *testing.T) {
	provider := New("test-project")

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

	_, err = provider.MergePullRequest("test-repo", "test-branch")
	if err == nil {
		t.Log("MergePullRequest succeeded (unexpected without auth)")
	} else {
		t.Logf("MergePullRequest failed as expected: %v", err)
	}
}

func TestBitbucketProviderRegistration(t *testing.T) {
	// Test that the Bitbucket provider is registered during init
	provider := scm.Get("bitbucket", "test-project")

	if provider == nil {
		t.Fatal("Expected Bitbucket provider to be registered")
	}

	// Verify it's the correct type
	_, ok := provider.(*Bitbucket)
	if !ok {
		t.Errorf("Expected *Bitbucket provider, got %T", provider)
	}
}

func TestBitbucketURL(t *testing.T) {
	bitbucket := &Bitbucket{
		host:    "bitbucket.example.com",
		project: "TEST",
	}

	tests := []struct {
		name     string
		repo     string
		path     []string
		expected string
	}{
		{
			name:     "ProjectOnly",
			repo:     "",
			path:     nil,
			expected: "https:%2F%2Fbitbucket.example.com%2Frest%2Fapi%2F1.0%2Fprojects%2FTEST",
		},
		{
			name:     "ProjectWithRepo",
			repo:     "my-repo",
			path:     nil,
			expected: "https:%2F%2Fbitbucket.example.com%2Frest%2Fapi%2F1.0%2Fprojects%2FTEST/repos/my-repo",
		},
		{
			name:     "ProjectWithRepoAndPath",
			repo:     "my-repo",
			path:     []string{"pull-requests"},
			expected: "https:%2F%2Fbitbucket.example.com%2Frest%2Fapi%2F1.0%2Fprojects%2FTEST/repos/my-repo/pull-requests",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := bitbucket.url(test.repo, test.path...)
			if actual != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, actual)
			}
		})
	}
}
