package github

import (
	"errors"
	"testing"

	"github.com/ryclarke/batch-tool/auth"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

// TestNewResolvesCredentialPerProject covers the motivating case for per-owner
// credentials: two projects on the same host authenticating separately.
func TestNewResolvesCredentialPerProject(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProvider, "github")
	viper.Set(config.AuthProvider, auth.BackendEnv)
	viper.Set(config.AuthOwners, map[string]any{
		"personal": map[string]any{"env": "PERSONAL_TOKEN"},
		"work":     map[string]any{"env": "WORK_TOKEN"},
	})

	t.Setenv("PERSONAL_TOKEN", "personal-secret")
	t.Setenv("WORK_TOKEN", "work-secret")
	auth.Reset()

	for _, project := range []string{"personal", "work"} {
		provider, ok := New(ctx, project).(*Github)
		if !ok {
			t.Fatalf("Expected *Github provider for project %s", project)
		}

		if provider.authErr != nil {
			t.Errorf("Project %s should resolve a credential, got: %v", project, provider.authErr)
		}
	}
}

// TestNewRecordsCredentialFailure verifies that a resolution failure survives the
// scm.ProviderFactory signature, which cannot return an error.
func TestNewRecordsCredentialFailure(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProvider, "github")
	viper.Set(config.AuthProvider, auth.BackendEnv)
	viper.Set(config.AuthEnv, "ABSENT_TOKEN")
	auth.Reset()

	provider, ok := New(ctx, "test-project").(*Github)
	if !ok {
		t.Fatal("Expected *Github provider")
	}

	if provider.authErr == nil {
		t.Fatal("Expected New to record a credential resolution failure")
	}

	// Every method which issues a request must surface the failure rather than
	// attempting an unauthenticated call.
	t.Run("ListRepositories", func(t *testing.T) {
		_, err := provider.ListRepositories()
		assertCredentialError(t, err)
	})

	t.Run("GetPullRequest", func(t *testing.T) {
		_, err := provider.GetPullRequest("repo", "branch")
		assertCredentialError(t, err)
	})

	t.Run("OpenPullRequest", func(t *testing.T) {
		_, err := provider.OpenPullRequest("repo", "branch", &scm.PROptions{})
		assertCredentialError(t, err)
	})

	t.Run("UpdatePullRequest", func(t *testing.T) {
		_, err := provider.UpdatePullRequest("repo", "branch", &scm.PROptions{})
		assertCredentialError(t, err)
	})

	t.Run("MergePullRequest", func(t *testing.T) {
		_, err := provider.MergePullRequest("repo", "branch", &scm.PRMergeOptions{})
		assertCredentialError(t, err)
	})

	// CheckCapabilities is a local check against static capabilities and needs no
	// credential, so it must still work.
	t.Run("CheckCapabilities", func(t *testing.T) {
		if err := provider.CheckCapabilities(&scm.PROptions{Draft: new(bool)}); err != nil {
			t.Errorf("CheckCapabilities should not require a credential, got: %v", err)
		}
	})
}

func assertCredentialError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("Expected a credential error")
	}

	if !errors.Is(err, auth.ErrNoCredential) {
		t.Errorf("error = %v, want ErrNoCredential", err)
	}
}
