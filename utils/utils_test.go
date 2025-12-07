package utils

import (
	"context"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func loadFixture(t *testing.T) context.Context {
	return config.LoadFixture(t, "../config")
}

func TestCleanFilter(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name       string
		filter     string
		wantResult string
	}{
		{
			name:       "remove label token",
			filter:     "~frontend",
			wantResult: "frontend",
		},
		{
			name:       "remove skip token",
			filter:     "!backend",
			wantResult: "backend",
		},
		{
			name:       "remove forced token",
			filter:     "+deprecated",
			wantResult: "deprecated",
		},
		{
			name:       "remove multiple tokens",
			filter:     "+~frontend",
			wantResult: "frontend",
		},
		{
			name:       "remove skip and label tokens",
			filter:     "!~backend",
			wantResult: "backend",
		},
		{
			name:       "plain name unchanged",
			filter:     "web-app",
			wantResult: "web-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanFilter(ctx, tt.filter)
			checkStringEqual(t, result, tt.wantResult)
		})
	}
}

func TestValidateRequiredConfig(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	tests := []struct {
		name      string
		setup     func()
		keys      []string
		wantError bool
	}{
		{
			name:      "missing required config",
			keys:      []string{"nonexistent.key"},
			wantError: true,
		},
		{
			name: "existing config",
			setup: func() {
				viper.Set("test.key", "test-value")
			},
			keys:      []string{"test.key"},
			wantError: false,
		},
		{
			name: "multiple keys some missing",
			setup: func() {
				viper.Set("test.key", "test-value")
			},
			keys:      []string{"test.key", "missing.key"},
			wantError: true,
		},
		{
			name: "multiple keys all present",
			setup: func() {
				viper.Set("test.key", "test-value")
				viper.Set("test.key2", "test-value2")
			},
			keys:      []string{"test.key", "test.key2"},
			wantError: false,
		},
		{
			name: "empty string value treated as missing",
			setup: func() {
				viper.Set("empty.key", "")
			},
			keys:      []string{"empty.key"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			err := ValidateRequiredConfig(ctx, tt.keys...)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateRequiredConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

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
			host, project, name := ParseRepo(ctx, tt.repo)

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
			got := RepoPath(ctx, tt.repo)
			if got != tt.wantPath {
				t.Errorf("RepoPath() = %v, want %v", got, tt.wantPath)
			}
		})
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

			url := RepoURL(ctx, tt.repo)
			for _, want := range tt.wantContains {
				checkStringContains(t, url, want)
			}
		})
	}
}

// Note: LookupBranch and ValidateBranch functions require git repository setup
// and would be better tested in integration tests rather than unit tests
// since they execute git commands.
