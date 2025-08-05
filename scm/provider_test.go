package scm_test

import (
	"errors"
	"testing"

	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
)

func TestRegister(t *testing.T) {
	// Register a test provider using fake package
	scm.Register("test-provider", func(project string) scm.Provider {
		return fake.New(project)
	})

	// Test successful retrieval
	provider := scm.Get("test-provider", "test-project")
	if provider == nil {
		t.Fatal("Expected provider to be returned")
	}

	// Test that the provider works correctly
	repos, err := provider.ListRepositories()
	if err != nil {
		t.Errorf("ListRepositories failed: %v", err)
	}
	if repos == nil {
		t.Error("Expected non-nil repositories slice")
	}
}

func TestGet(t *testing.T) {
	// Register a test provider with data
	scm.Register("test-scm-with-data", func(project string) scm.Provider {
		return fake.NewFake(project, fake.CreateTestRepositories(project))
	})

	// Test successful retrieval
	provider := scm.Get("test-scm-with-data", "my-project")
	if provider == nil {
		t.Fatal("Expected provider to be returned")
	}

	// Test that the provider has the expected data
	repos, err := provider.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories failed: %v", err)
	}

	if len(repos) != 5 {
		t.Errorf("Expected 5 repositories, got %d", len(repos))
	}

	// Verify the first repository
	repo := repos[0]
	if repo.Name != "repo-1" {
		t.Errorf("Expected first repository to be 'repo-1', got %s", repo.Name)
	}
	if repo.Project != "my-project" {
		t.Errorf("Expected project to be 'my-project', got %s", repo.Project)
	}
}

func TestGetPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when getting unregistered provider")
		} else {
			expectedMsg := "SCM provider nonexistent not registered"
			if r != expectedMsg {
				t.Errorf("Expected panic message '%s', got '%v'", expectedMsg, r)
			}
		}
	}()

	scm.Get("nonexistent", "project")
}

func TestProviderInterface(t *testing.T) {
	// Register a provider for testing
	scm.Register("interface-test", func(project string) scm.Provider {
		return fake.New(project)
	})

	provider := scm.Get("interface-test", "test-project")

	// Test all interface methods exist and can be called
	repos, err := provider.ListRepositories()
	if err != nil {
		t.Errorf("ListRepositories failed: %v", err)
	}
	if repos == nil {
		t.Error("Expected non-nil repositories slice")
	}

	// Test GetPullRequest (should fail for non-existent PR)
	_, err = provider.GetPullRequest("repo", "branch")
	if err == nil {
		t.Error("Expected error for non-existent pull request")
	}

	// For testing PR operations, create a new provider with a repository
	testProvider := fake.New("test-project").(*fake.Fake)
	testProvider.AddRepository(&scm.Repository{
		Name:    "test-repo",
		Project: "test-project",
		Labels:  []string{"test"},
	})

	// Test OpenPullRequest
	pr, err := testProvider.OpenPullRequest("test-repo", "feature-branch", "Test PR", "Description", []string{"reviewer1"})
	if err != nil {
		t.Errorf("OpenPullRequest failed: %v", err)
	}
	if pr == nil {
		t.Error("Expected non-nil pull request")
	}

	// Test UpdatePullRequest
	updatedPR, err := testProvider.UpdatePullRequest("test-repo", "feature-branch", "Updated Title", "Updated Description", []string{"reviewer2"}, false)
	if err != nil {
		t.Errorf("UpdatePullRequest failed: %v", err)
	}
	if updatedPR["title"] != "Updated Title" {
		t.Errorf("Expected title to be updated, got %v", updatedPR["title"])
	}

	// Test MergePullRequest
	mergedPR, err := testProvider.MergePullRequest("test-repo", "feature-branch")
	if err != nil {
		t.Errorf("MergePullRequest failed: %v", err)
	}
	if mergedPR["state"] != "MERGED" {
		t.Errorf("Expected PR to be merged, got state %v", mergedPR["state"])
	}
}

func TestProviderFactory(t *testing.T) {
	// Test that ProviderFactory type works correctly
	factory := func(project string) scm.Provider {
		return fake.New(project)
	}

	provider := factory("test-project")
	if provider == nil {
		t.Error("Expected non-nil provider from factory")
	}

	// Test that the provider works
	repos, err := provider.ListRepositories()
	if err != nil {
		t.Errorf("Factory provider failed: %v", err)
	}
	if repos == nil {
		t.Error("Expected non-nil repositories from factory provider")
	}
}

func TestErrorProviderIntegration(t *testing.T) {
	// Register an error provider
	scm.Register("error-provider", func(project string) scm.Provider {
		provider := fake.New(project).(*fake.Fake)
		provider.SetError("ListRepositories", errors.New("API unavailable"))
		return provider
	})

	provider := scm.Get("error-provider", "error-project")

	// Test that errors propagate correctly
	_, err := provider.ListRepositories()
	if err == nil {
		t.Error("Expected error from provider")
	}
	if err.Error() != "API unavailable" {
		t.Errorf("Expected 'API unavailable', got %v", err)
	}
}

func TestMultipleProviders(t *testing.T) {
	// Register multiple providers
	scm.Register("provider-a", func(project string) scm.Provider {
		return fake.NewFake("project-a-"+project, fake.CreateTestRepositories("project-a-"+project))
	})

	scm.Register("provider-b", func(project string) scm.Provider {
		return fake.NewFake("project-b-"+project, fake.CreateTestRepositories("project-b-"+project))
	})

	// Test both providers work independently
	providerA := scm.Get("provider-a", "test")
	providerB := scm.Get("provider-b", "test")

	reposA, err := providerA.ListRepositories()
	if err != nil {
		t.Errorf("Provider A failed: %v", err)
	}

	reposB, err := providerB.ListRepositories()
	if err != nil {
		t.Errorf("Provider B failed: %v", err)
	}

	if len(reposA) != len(reposB) {
		t.Error("Expected both providers to have same number of test repositories")
	}

	// Verify they have different projects
	if reposA[0].Project == reposB[0].Project {
		t.Error("Expected providers to have different projects")
	}
}

func TestDuplicateRegistration(t *testing.T) {
	// Test that duplicate registration doesn't overwrite existing provider
	originalFactory := func(project string) scm.Provider {
		provider := fake.New(project).(*fake.Fake)
		provider.AddRepository(&scm.Repository{Name: "original-repo", Project: project})
		return provider
	}

	newFactory := func(project string) scm.Provider {
		provider := fake.New(project).(*fake.Fake)
		provider.AddRepository(&scm.Repository{Name: "new-repo", Project: project})
		return provider
	}

	// Register first provider
	scm.Register("duplicate-test", originalFactory)

	// Get provider and verify it's the original
	provider1 := scm.Get("duplicate-test", "test-project")
	repos1, _ := provider1.ListRepositories()
	if len(repos1) != 1 || repos1[0].Name != "original-repo" {
		t.Error("Expected original provider to be preserved")
	}

	// Try to register duplicate - should be ignored
	scm.Register("duplicate-test", newFactory)

	// Get provider again and verify it's still the original
	provider2 := scm.Get("duplicate-test", "test-project")
	repos2, _ := provider2.ListRepositories()
	if len(repos2) != 1 || repos2[0].Name != "original-repo" {
		t.Error("Expected original provider to be preserved after duplicate registration")
	}
}
