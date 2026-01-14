package utils_test

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

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
			result := utils.CleanFilter(ctx, tt.filter)
			testhelper.AssertEqual(t, result, tt.wantResult)
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

			err := utils.ValidateRequiredConfig(ctx, tt.keys...)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateRequiredConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateEnumConfig(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		configValue  string
		validChoices []string
		expectError  bool
	}{
		{
			name:         "valid choice",
			key:          "output-style",
			configValue:  "json",
			validChoices: []string{"json", "yaml", "text"},
			expectError:  false,
		},
		{
			name:         "empty value - should pass",
			key:          "output-style",
			configValue:  "",
			validChoices: []string{"json", "yaml", "text"},
			expectError:  false,
		},
		{
			name:         "invalid choice",
			key:          "output-style",
			configValue:  "xml",
			validChoices: []string{"json", "yaml", "text"},
			expectError:  true,
		},
		{
			name:         "single valid choice",
			key:          "format",
			configValue:  "table",
			validChoices: []string{"table"},
			expectError:  false,
		},
		{
			name:         "invalid with single choice",
			key:          "format",
			configValue:  "list",
			validChoices: []string{"table"},
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			// Set config value
			if tt.configValue != "" {
				viper.Set(tt.key, tt.configValue)
			}

			// Create command
			cmd := &cobra.Command{Use: "test"}
			cmd.SetContext(ctx)

			// Run check
			err := utils.ValidateEnumConfig(cmd, tt.key, tt.validChoices)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), "invalid") {
					t.Errorf("Error should contain 'invalid', got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestExecEnv(t *testing.T) {
	tests := []struct {
		name     string
		repo     string
		setup    func(t *testing.T, ctx context.Context)
		validate func(t *testing.T, env []string)
	}{
		{
			name: "basic repo environment variables",
			repo: "test-repo",
			setup: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "default-project")
				viper.Set(config.Branch, "main")
				// Setup repo directory for LookupBranch
				testhelper.SetupDirs(t, ctx, []string{"test-repo"})
			},
			validate: func(t *testing.T, env []string) {
				testhelper.AssertContains(t, env, "REPO_NAME=test-repo")
				testhelper.AssertContains(t, env, "GIT_BRANCH=main")
				testhelper.AssertContains(t, env, "GIT_PROJECT=default-project")
			},
		},
		{
			name: "includes system environment variables",
			repo: "my-app",
			setup: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "my-project")
				viper.Set(config.Branch, "develop")
				testhelper.SetupDirs(t, ctx, []string{"my-app"})
			},
			validate: func(t *testing.T, env []string) {
				// Verify the environment contains at least the system PATH
				found := false
				for _, e := range env {
					if strings.HasPrefix(e, "PATH=") {
						found = true
						break
					}
				}
				if !found {
					t.Error("environment should include system variables like PATH")
				}
			},
		},
		{
			name: "with custom environment variables",
			repo: "service",
			setup: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "backend")
				viper.Set(config.Branch, "feature/test")
				viper.Set(config.CmdEnv, []string{"CUSTOM_VAR=custom-value", "DEBUG=true"})
				testhelper.SetupDirs(t, ctx, []string{"service"})
			},
			validate: func(t *testing.T, env []string) {
				testhelper.AssertContains(t, env, "REPO_NAME=service")
				testhelper.AssertContains(t, env, "GIT_BRANCH=feature/test")
				testhelper.AssertContains(t, env, "GIT_PROJECT=backend")
				testhelper.AssertContains(t, env, "CUSTOM_VAR=custom-value")
				testhelper.AssertContains(t, env, "DEBUG=true")
			},
		},
		{
			name: "multiple custom environment variables",
			repo: "another-repo",
			setup: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "test-project")
				viper.Set(config.Branch, "main")
				viper.Set(config.CmdEnv, []string{
					"ENV_A=value-a",
					"ENV_B=value-b",
					"ENV_C=value-c",
				})
				testhelper.SetupDirs(t, ctx, []string{"another-repo"})
			},
			validate: func(t *testing.T, env []string) {
				testhelper.AssertContains(t, env, "ENV_A=value-a")
				testhelper.AssertContains(t, env, "ENV_B=value-b")
				testhelper.AssertContains(t, env, "ENV_C=value-c")
			},
		},
		{
			name: "no custom environment variables",
			repo: "simple",
			setup: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "proj")
				viper.Set(config.Branch, "master")
				// Don't set CmdEnv - should be empty or not set
				testhelper.SetupDirs(t, ctx, []string{"simple"})
			},
			validate: func(t *testing.T, env []string) {
				testhelper.AssertContains(t, env, "REPO_NAME=simple")
				testhelper.AssertContains(t, env, "GIT_BRANCH=master")
				testhelper.AssertContains(t, env, "GIT_PROJECT=proj")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			if tt.setup != nil {
				tt.setup(t, ctx)
			}

			env := utils.ExecEnv(ctx, tt.repo)

			// Verify it returns a slice (not nil)
			if env == nil {
				t.Fatal("ExecEnv returned nil")
			}

			// Should have at least system env vars + 3 required vars
			if len(env) < 3 {
				t.Errorf("ExecEnv returned %d vars, expected at least 3", len(env))
			}

			// Run validation
			if tt.validate != nil {
				tt.validate(t, env)
			}
		})
	}
}
