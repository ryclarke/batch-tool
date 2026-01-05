package testing

import (
	"context"
	"os"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
)

// SetupFakeProvider creates and registers a fake SCM provider with test data
func SetupFakeProvider(t *testing.T, _ context.Context, providerName string, project string) scm.Provider {
	t.Helper()

	provider := fake.NewFake(project, fake.CreateTestRepositories(project))
	scm.Register(providerName, func(_ context.Context, _ string) scm.Provider {
		return provider
	})

	return provider
}

// SetupFakeProviderWithRepos creates a fake provider with custom repositories
func SetupFakeProviderWithRepos(t *testing.T, _ context.Context, providerName string, project string, repos []*scm.Repository) scm.Provider {
	t.Helper()

	provider := fake.NewFake(project, repos)
	scm.Register(providerName, func(_ context.Context, _ string) scm.Provider {
		return provider
	})

	return provider
}

// SetupMultipleFakeProviders creates fake providers for multiple projects
func SetupMultipleFakeProviders(t *testing.T, _ context.Context, projects []string) map[string]scm.Provider {
	t.Helper()

	providers := make(map[string]scm.Provider)

	for _, project := range projects {
		providerName := "fake-" + project
		provider := fake.NewFake(project, fake.CreateTestRepositories(project))

		// Capture the provider in closure properly
		p := provider
		scm.Register(providerName, func(_ context.Context, _ string) scm.Provider {
			return p
		})

		providers[project] = provider
	}

	return providers
}

// SetupRealProvider checks for credentials and skips test if not available
// This is used for optional integration tests against real SCM providers
func SetupRealProvider(t *testing.T, ctx context.Context, providerName string) {
	t.Helper()

	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	var envVar string
	switch providerName {
	case "github":
		envVar = "GITHUB_TEST_TOKEN"
	case "bitbucket":
		envVar = "BITBUCKET_TEST_TOKEN"
	default:
		t.Fatalf("Unknown provider: %s", providerName)
	}

	token := os.Getenv(envVar)
	if token == "" {
		t.Skipf("%s not set, skipping integration test", envVar)
	}

	viper := config.Viper(ctx)
	viper.Set(config.AuthToken, token)
}

// AssertCatalogState validates the state of the catalog
func AssertCatalogState(t *testing.T, catalog map[string]scm.Repository, labels map[string]mapset.Set[string], expectedRepos int, minLabels int) {
	t.Helper()

	if len(catalog) != expectedRepos {
		t.Errorf("Expected %d repositories in catalog, got %d", expectedRepos, len(catalog))
	}

	if len(labels) < minLabels {
		t.Errorf("Expected at least %d labels, got %d", minLabels, len(labels))
	}

	// Verify catalog integrity
	for key, repo := range catalog {
		if repo.Name == "" {
			t.Errorf("Repository %s has empty name", key)
		}
		if repo.Project == "" {
			t.Errorf("Repository %s has empty project", key)
		}
	}

	// Verify label integrity
	for labelName, repos := range labels {
		if repos == nil {
			t.Errorf("Label %s has nil repository set", labelName)
		}
		if repos.Cardinality() == 0 {
			t.Errorf("Label %s has no repositories", labelName)
		}
	}
}

// AssertRepositoryExists checks if a repository exists in the catalog
func AssertRepositoryExists(t *testing.T, catalog map[string]scm.Repository, repoKey string) {
	t.Helper()

	if _, exists := catalog[repoKey]; !exists {
		t.Errorf("Expected repository %s to exist in catalog", repoKey)
	}
}

// AssertLabelExists checks if a label exists and optionally validates repository count
func AssertLabelExists(t *testing.T, labels map[string]mapset.Set[string], labelName string, expectedCount int) {
	t.Helper()

	labelSet, exists := labels[labelName]
	if !exists {
		t.Errorf("Expected label %s to exist", labelName)
		return
	}

	if expectedCount >= 0 && labelSet.Cardinality() != expectedCount {
		t.Errorf("Expected label %s to have %d repositories, got %d", labelName, expectedCount, labelSet.Cardinality())
	}
}

// AssertLabelContainsRepo checks if a label contains a specific repository
func AssertLabelContainsRepo(t *testing.T, labels map[string]mapset.Set[string], labelName string, repoKey string) {
	t.Helper()

	labelSet, exists := labels[labelName]
	if !exists {
		t.Errorf("Label %s does not exist", labelName)
		return
	}

	if !labelSet.Contains(repoKey) {
		t.Errorf("Expected label %s to contain repository %s", labelName, repoKey)
	}
}

// CreateTestRepository creates a repository with specified attributes
func CreateTestRepository(name, project string, labels []string) *scm.Repository {
	return &scm.Repository{
		Name:          name,
		Project:       project,
		Description:   "Test repository " + name,
		DefaultBranch: "main",
		Labels:        labels,
	}
}

// CreateTestRepositories creates multiple test repositories
func CreateTestRepositories(count int, project string, labelPrefix string) []*scm.Repository {
	repos := make([]*scm.Repository, 0, count)

	for i := 1; i <= count; i++ {
		name := "test-repo-" + string(rune('0'+i))
		repos = append(repos, CreateTestRepository(name, project, []string{labelPrefix + "-label"}))
	}

	return repos
}
