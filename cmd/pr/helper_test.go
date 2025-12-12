package pr

import (
	"context"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

// loadFixture loads the test configuration
func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../../config")
}

// setupTestContext configures a test context with fake SCM provider
// Returns the provider instance that will be used by all scm.Get calls
func setupTestContext(t *testing.T, reposPath string) (context.Context, *fake.Fake) {
	t.Helper()

	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Use a unique provider name for each test to avoid state sharing
	providerName := "fake-test-" + t.Name()

	// Configure viper for testing
	viper.Set(config.GitProvider, providerName)
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.AuthToken, "fake-token")
	viper.Set(config.GitDirectory, reposPath)
	viper.Set(config.GitHost, "example.com")
	viper.Set(config.Branch, "feature-branch")
	viper.Set(config.MaxConcurrency, 1) // Serial execution for predictable test output
	viper.Set(config.ChannelBuffer, 10)

	// Create test repositories
	testRepos := []*scm.Repository{
		{
			Name:          "repo-1",
			Description:   "Test Repository 1",
			Project:       "test-project",
			DefaultBranch: "main",
			Labels:        []string{"test"},
		},
		{
			Name:          "repo-2",
			Description:   "Test Repository 2",
			Project:       "test-project",
			DefaultBranch: "main",
			Labels:        []string{"test"},
		},
	}

	// Create a single provider instance that will be shared
	sharedProvider := fake.NewFake("test-project", testRepos)

	// Register fake provider factory that returns the same instance each time
	scm.Register(providerName, func(ctx context.Context, project string) scm.Provider {
		return sharedProvider
	})

	return ctx, sharedProvider
}
