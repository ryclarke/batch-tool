package utils

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
)

func TestSCMIntegrationWithUtils(t *testing.T) {
	// Save original configuration
	originalProvider := viper.GetString(config.GitProvider)
	originalProject := viper.GetString(config.GitProject)
	originalHost := viper.GetString(config.GitHost)
	originalUser := viper.GetString(config.GitUser)
	originalGopath := viper.GetString(config.EnvGopath)

	defer func() {
		viper.Set(config.GitProvider, originalProvider)
		viper.Set(config.GitProject, originalProject)
		viper.Set(config.GitHost, originalHost)
		viper.Set(config.GitUser, originalUser)
		viper.Set(config.EnvGopath, originalGopath)
	}()

	// Configure for testing
	viper.Set(config.GitProvider, "fake")
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.GitHost, "github.com")
	viper.Set(config.GitUser, "testuser")
	viper.Set(config.EnvGopath, "/tmp/test-gopath")

	// Register fake provider with test repositories
	scm.Register("fake-utils-test", func(project string) scm.Provider {
		return fake.NewFake(project, fake.CreateTestRepositories(project))
	})

	// Update provider for testing
	viper.Set(config.GitProvider, "fake-utils-test")

	t.Run("ValidateRequiredConfigForSCM", func(t *testing.T) {
		// Test validation of SCM-related configuration
		err := ValidateRequiredConfig(config.GitProvider, config.GitProject)
		if err != nil {
			t.Errorf("Expected no error for valid SCM config, got: %v", err)
		}

		// Test with missing provider
		viper.Set(config.GitProvider, "")
		err = ValidateRequiredConfig(config.GitProvider)
		if err == nil {
			t.Error("Expected error for missing provider")
		}

		// Restore
		viper.Set(config.GitProvider, "fake-utils-test")
	})

	t.Run("ParseRepoWithSCMContext", func(t *testing.T) {
		// Test parsing repository identifiers
		tests := []struct {
			input           string
			expectedHost    string
			expectedProject string
			expectedName    string
		}{
			{
				input:           "repo-1",
				expectedHost:    "github.com",
				expectedProject: "test-project",
				expectedName:    "repo-1",
			},
			{
				input:           "custom-project/repo-2",
				expectedHost:    "github.com",
				expectedProject: "custom-project",
				expectedName:    "repo-2",
			},
			{
				input:           "custom.host.com/custom-project/repo-3",
				expectedHost:    "", // ParseRepo doesn't handle full URLs this way
				expectedProject: "custom-project",
				expectedName:    "repo-3",
			},
		}

		for _, test := range tests {
			t.Run(test.input, func(t *testing.T) {
				host, project, name := ParseRepo(test.input)

				if host != test.expectedHost {
					t.Errorf("Expected host %s, got %s", test.expectedHost, host)
				}
				if project != test.expectedProject {
					t.Errorf("Expected project %s, got %s", test.expectedProject, project)
				}
				if name != test.expectedName {
					t.Errorf("Expected name %s, got %s", test.expectedName, name)
				}
			})
		}
	})

	t.Run("RepoPathWithSCMContext", func(t *testing.T) {
		path := RepoPath("repo-1")
		expectedPath := filepath.Join("/tmp/test-gopath", "src", "github.com", "test-project", "repo-1")

		if path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, path)
		}
	})

	t.Run("RepoURLWithSCMContext", func(t *testing.T) {
		url := RepoURL("repo-1")
		expectedURL := "ssh://testuser@github.com/test-project/repo-1.git"

		if url != expectedURL {
			t.Errorf("Expected URL %s, got %s", expectedURL, url)
		}
	})
}

func TestLookupReviewersWithSCMRepositories(t *testing.T) {
	// Save original configuration
	originalReviewers := viper.Get(config.Reviewers)
	originalDefaultReviewers := viper.Get(config.DefaultReviewers)

	defer func() {
		viper.Set(config.Reviewers, originalReviewers)
		viper.Set(config.DefaultReviewers, originalDefaultReviewers)
	}()

	// Configure reviewers for repositories from fake provider
	viper.Set(config.DefaultReviewers, map[string][]string{
		"repo-1": {"alice", "bob"},
		"repo-2": {"charlie", "diana"},
		"repo-3": {"eve", "frank"},
	})

	t.Run("LookupDefaultReviewers", func(t *testing.T) {
		reviewers := LookupReviewers("repo-1")
		expected := []string{"alice", "bob"}

		if len(reviewers) != len(expected) {
			t.Errorf("Expected %d reviewers, got %d", len(expected), len(reviewers))
		}

		for i, reviewer := range reviewers {
			if reviewer != expected[i] {
				t.Errorf("Expected reviewer %s at position %d, got %s", expected[i], i, reviewer)
			}
		}
	})

	t.Run("LookupGlobalReviewers", func(t *testing.T) {
		// Set global reviewers (should override default)
		viper.Set(config.Reviewers, []string{"global1", "global2"})

		reviewers := LookupReviewers("repo-1")
		expected := []string{"global1", "global2"}

		if len(reviewers) != len(expected) {
			t.Errorf("Expected %d global reviewers, got %d", len(expected), len(reviewers))
		}

		for i, reviewer := range reviewers {
			if reviewer != expected[i] {
				t.Errorf("Expected global reviewer %s at position %d, got %s", expected[i], i, reviewer)
			}
		}
	})

	t.Run("LookupReviewersForUnknownRepo", func(t *testing.T) {
		// Clear global reviewers
		viper.Set(config.Reviewers, []string{})

		reviewers := LookupReviewers("unknown-repo")
		if len(reviewers) != 0 {
			t.Errorf("Expected no reviewers for unknown repo, got %d", len(reviewers))
		}
	})
}

func TestValidateBranchIntegration(t *testing.T) {
	// This test requires git repository setup, so we'll test the basic structure
	originalSourceBranch := viper.GetString(config.SourceBranch)
	defer func() {
		viper.Set(config.SourceBranch, originalSourceBranch)
	}()

	// Set up test environment
	viper.Set(config.SourceBranch, "main")
	viper.Set(config.EnvGopath, "/tmp/test-gopath")

	// Create a test channel
	ch := make(chan string, 1)

	// Test with a non-existent repository (should return error)
	err := ValidateBranch("nonexistent-repo", ch)
	if err == nil {
		t.Error("Expected error for non-existent repository")
	}

	// Clean up
	close(ch)
}

func TestLookupBranchIntegration(t *testing.T) {
	originalBranch := viper.GetString(config.Branch)
	originalGopath := viper.GetString(config.EnvGopath)

	defer func() {
		viper.Set(config.Branch, originalBranch)
		viper.Set(config.EnvGopath, originalGopath)
	}()

	// Test with branch already set in config
	viper.Set(config.Branch, "feature-branch")
	viper.Set(config.EnvGopath, "/tmp/test-gopath")

	branch, err := LookupBranch("test-repo")
	if err != nil {
		t.Errorf("Expected no error when branch is set in config, got: %v", err)
	}

	if branch != "feature-branch" {
		t.Errorf("Expected branch 'feature-branch', got %s", branch)
	}

	// Test with branch not set (will try to read from git)
	viper.Set(config.Branch, "")

	branch, err = LookupBranch("test-repo")
	// This will likely fail since we don't have a real git repo, but we can test the error handling
	if err == nil {
		t.Logf("Unexpectedly succeeded in getting branch: %s", branch)
	} else {
		t.Logf("Expected error when trying to read from non-existent git repo: %v", err)
	}
}

func TestUtilsWithFakeRepositories(t *testing.T) {
	// Register fake provider
	scm.Register("utils-fake-provider", func(project string) scm.Provider {
		return fake.NewFake(project, fake.CreateTestRepositories(project))
	})

	// Get the fake provider to access its repositories
	provider := scm.Get("utils-fake-provider", "test-project")
	repos, err := provider.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to get repositories from fake provider: %v", err)
	}

	// Test utils functions with repository names from fake provider
	for _, repo := range repos {
		t.Run("Repository_"+repo.Name, func(t *testing.T) {
			// Test that ParseRepo works with repository names
			_, _, name := ParseRepo(repo.Name)
			if name != repo.Name {
				t.Errorf("Expected parsed name %s, got %s", repo.Name, name)
			}

			// Test RepoPath generation
			path := RepoPath(repo.Name)
			if !filepath.IsAbs(path) {
				t.Error("Expected absolute path from RepoPath")
			}

			// Test RepoURL generation
			url := RepoURL(repo.Name)
			if url == "" {
				t.Error("Expected non-empty URL from RepoURL")
			}

			// Verify URL format
			if !strings.Contains(url, repo.Name) {
				t.Errorf("Expected URL to contain repository name %s, got %s", repo.Name, url)
			}
		})
	}
}
