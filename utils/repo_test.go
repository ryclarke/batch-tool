package utils_test

import (
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestParseRepo(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Set up default config
	viper.Set(config.GitHost, "github.com")
	viper.Set(config.GitProject, "default-project")

	tests := []struct {
		name        string
		repo        string
		wantHost    string
		wantProject string
		wantName    string
	}{
		{
			name:        "simple repo name",
			repo:        "my-repo",
			wantHost:    "github.com",
			wantProject: "default-project",
			wantName:    "my-repo",
		},
		{
			name:        "project/repo format",
			repo:        "custom-project/my-repo",
			wantHost:    "github.com",
			wantProject: "custom-project",
			wantName:    "my-repo",
		},
		{
			name:        "full host/project/repo format",
			repo:        "example.com/custom-project/my-repo",
			wantProject: "custom-project",
			wantName:    "my-repo",
		},
		{
			name:        "with leading/trailing slashes",
			repo:        "/custom-project/my-repo/",
			wantProject: "custom-project",
			wantName:    "my-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, project, name := utils.ParseRepo(ctx, tt.repo)

			if tt.wantHost != "" && host != tt.wantHost {
				t.Errorf("ParseRepo() host = %v, want %v", host, tt.wantHost)
			}
			if project != tt.wantProject {
				t.Errorf("ParseRepo() project = %v, want %v", project, tt.wantProject)
			}
			if name != tt.wantName {
				t.Errorf("ParseRepo() name = %v, want %v", name, tt.wantName)
			}
		})
	}
}

func TestRepoPath(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Set up config
	viper.Set(config.GitDirectory, "/test/gitdir/src")
	viper.Set(config.GitHost, "github.com")
	viper.Set(config.GitProject, "test-project")

	tests := []struct {
		name     string
		repo     string
		wantPath string
	}{
		{
			name:     "simple repo name",
			repo:     "my-repo",
			wantPath: "/test/gitdir/src/github.com/test-project/my-repo",
		},
		{
			name:     "custom project",
			repo:     "custom-project/my-repo",
			wantPath: "/test/gitdir/src/github.com/custom-project/my-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := utils.RepoPath(ctx, tt.repo)
			if got != tt.wantPath {
				t.Errorf("RepoPath() = %v, want %v", got, tt.wantPath)
			}
		})
	}
}

func TestRepoPathEmptyRepo(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitDirectory, "/test/gitdir/src")

	got := utils.RepoPath(ctx, "")

	// Should return absolute path to git directory
	if got != "/test/gitdir/src" {
		t.Errorf("RepoPath(\"\") = %v, want /test/gitdir/src", got)
	}
}

func TestRepoURL(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name         string
		repo         string
		wantContains []string
	}{
		{
			name:         "generates URL with host, project, and repo",
			repo:         "my-repo",
			wantContains: []string{"github.com", "test-project", "my-repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper := config.Viper(ctx)
			viper.Set(config.GitUser, "git")
			viper.Set(config.GitHost, "github.com")
			viper.Set(config.GitProject, "test-project")

			url := utils.RepoURL(ctx, tt.repo)
			for _, want := range tt.wantContains {
				testhelper.AssertContains(t, url, want)
			}
		})
	}
}

// Note: LookupBranch and ValidateBranch functions require git repository setup
// and would be better tested in integration tests rather than unit tests
// since they execute git commands.

func TestLookupBranchWithConfigSet(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// When branch is already in config, should return it without calling git
	viper.Set(config.Branch, "feature-branch")

	branch, err := utils.LookupBranch(ctx, "any-repo")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if branch != "feature-branch" {
		t.Errorf("LookupBranch() = %q, want \"feature-branch\"", branch)
	}
}
