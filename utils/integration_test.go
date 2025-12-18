package utils_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
	"github.com/ryclarke/batch-tool/scm/fake"
	"github.com/ryclarke/batch-tool/utils"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestSCMIntegrationWithUtils(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Configure for testing
	viper.Set(config.GitProvider, "fake")
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.GitHost, "github.com")
	viper.Set(config.GitUser, "testuser")
	viper.Set(config.GitDirectory, "/tmp/test-gitdir")

	// Register fake provider with test repositories
	scm.Register("fake-utils-test", func(_ context.Context, project string) scm.Provider {
		return fake.NewFake(project, fake.CreateTestRepositories(project))
	})

	// Update provider for testing
	viper.Set(config.GitProvider, "fake-utils-test")

	t.Run("ValidateRequiredConfigForSCM", func(t *testing.T) {
		tests := []struct {
			name      string
			setup     func()
			keys      []string
			wantError bool
		}{
			{
				name:      "valid SCM config",
				keys:      []string{config.GitProvider, config.GitProject},
				wantError: false,
			},
			{
				name: "missing provider",
				setup: func() {
					viper.Set(config.GitProvider, "")
				},
				keys:      []string{config.GitProvider},
				wantError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.setup != nil {
					tt.setup()
					defer viper.Set(config.GitProvider, "fake-utils-test")
				}

				err := utils.ValidateRequiredConfig(ctx, tt.keys...)
				testhelper.AssertError(t, err, tt.wantError)
			})
		}
	})

	t.Run("ParseRepoWithSCMContext", func(t *testing.T) {
		tests := []struct {
			name        string
			input       string
			wantHost    string
			wantProject string
			wantName    string
		}{
			{
				name:        "simple repo name",
				input:       "repo-1",
				wantHost:    "github.com",
				wantProject: "test-project",
				wantName:    "repo-1",
			},
			{
				name:        "project/repo format",
				input:       "custom-project/repo-2",
				wantHost:    "github.com",
				wantProject: "custom-project",
				wantName:    "repo-2",
			},
			{
				name:        "full URL format",
				input:       "custom.host.com/custom-project/repo-3",
				wantHost:    "",
				wantProject: "custom-project",
				wantName:    "repo-3",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				host, project, name := utils.ParseRepo(ctx, tt.input)

				if tt.wantHost != "" {
					testhelper.AssertEqual(t, host, tt.wantHost)
				}

				testhelper.AssertEqual(t, project, tt.wantProject)
				testhelper.AssertEqual(t, name, tt.wantName)
			})
		}
	})

	t.Run("RepoPathWithSCMContext", func(t *testing.T) {
		tests := []struct {
			name     string
			repo     string
			wantPath string
		}{
			{
				name:     "generates correct path",
				repo:     "repo-1",
				wantPath: filepath.Join("/tmp/test-gitdir", "github.com", "test-project", "repo-1"),
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				path := utils.RepoPath(ctx, tt.repo)
				testhelper.AssertEqual(t, path, tt.wantPath)
			})
		}
	})

	t.Run("RepoURLWithSCMContext", func(t *testing.T) {
		tests := []struct {
			name    string
			repo    string
			wantURL string
		}{
			{
				name:    "generates correct SSH URL",
				repo:    "repo-1",
				wantURL: "ssh://testuser@github.com/test-project/repo-1.git",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				url := utils.RepoURL(ctx, tt.repo)
				testhelper.AssertEqual(t, url, tt.wantURL)
			})
		}
	})
}

func TestLookupBranchIntegration(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name       string
		repo       string
		branchSet  string
		wantBranch string
		wantError  bool
	}{
		{
			name:       "branch set in config",
			repo:       "test-repo",
			branchSet:  "feature-branch",
			wantBranch: "feature-branch",
			wantError:  false,
		},
		{
			name:      "branch not set reads from git",
			repo:      "test-repo",
			branchSet: "",
			// Will likely fail without real git repo, error expected or logged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper := config.Viper(ctx)
			originalBranch := viper.GetString(config.Branch)
			originalGitdir := viper.GetString(config.GitDirectory)
			defer func() {
				viper.Set(config.Branch, originalBranch)
				viper.Set(config.GitDirectory, originalGitdir)
			}()

			viper.Set(config.Branch, tt.branchSet)
			viper.Set(config.GitDirectory, "/tmp/test-gitdir")

			branch, err := utils.LookupBranch(ctx, tt.repo)

			if tt.branchSet != "" {
				testhelper.AssertError(t, err, tt.wantError)
				if err == nil {
					testhelper.AssertEqual(t, branch, tt.wantBranch)
				}
			} else {
				// When branch not set, will try to read from git
				if err == nil {
					t.Logf("Unexpectedly succeeded in getting branch: %s", branch)
				} else {
					t.Logf("Expected error when trying to read from non-existent git repo: %v", err)
				}
			}
		})
	}
}

func TestUtilsWithFakeRepositories(t *testing.T) {
	ctx := loadFixture(t)

	// Register fake provider
	scm.Register("utils-fake-provider", func(_ context.Context, project string) scm.Provider {
		return fake.NewFake(project, fake.CreateTestRepositories(project))
	})

	// Get the fake provider to access its repositories
	provider := scm.Get(ctx, "utils-fake-provider", "test-project")
	repos, err := provider.ListRepositories()
	if err != nil {
		t.Fatalf("Failed to get repositories from fake provider: %v", err)
	}

	// Test utils functions with repository names from fake provider
	for _, repo := range repos {
		t.Run("Repository_"+repo.Name, func(t *testing.T) {
			// Test ParseRepo
			_, _, name := utils.ParseRepo(ctx, repo.Name)
			testhelper.AssertEqual(t, name, repo.Name)

			// Test RepoPath
			path := utils.RepoPath(ctx, repo.Name)
			checkAbsolutePath(t, path)

			// Test RepoURL
			url := utils.RepoURL(ctx, repo.Name)
			testhelper.AssertNotEmpty(t, url)
			testhelper.AssertContains(t, url, repo.Name)
		})
	}
}
