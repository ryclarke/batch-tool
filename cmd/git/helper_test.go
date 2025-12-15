package git

import (
	"context"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	testhelper "github.com/ryclarke/batch-tool/utils/test"
)

// loadFixture loads test configuration (local helper)
func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../../config")
}

// setupTestGitContext configures a test context for git commands
func setupTestGitContext(t *testing.T, reposPath string) context.Context {
	t.Helper()

	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Configure viper for testing
	viper.Set(config.GitDirectory, reposPath)
	viper.Set(config.GitHost, "example.com")
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.SourceBranch, "main")
	viper.Set(config.MaxConcurrency, 1) // Serial execution for predictable test output
	viper.Set(config.ChannelBuffer, 10)

	return ctx
}
