package utils_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func TestEnv(t *testing.T) {
	tests := []struct {
		name         string
		repo         string
		setupEnv     func(t *testing.T, ctx context.Context)
		validate     func(t *testing.T, env []string)
		expectError  bool
		errorMessage string
	}{
		{
			name: "basic repo environment variables",
			repo: "test-repo",
			setupEnv: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "default-project")
				viper.Set(config.Branch, "main")
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
			setupEnv: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "my-project")
				viper.Set(config.Branch, "develop")
				testhelper.SetupDirs(t, ctx, []string{"my-app"})
			},
			validate: func(t *testing.T, env []string) {
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
			name: "with custom key=value environment variables",
			repo: "service",
			setupEnv: func(t *testing.T, ctx context.Context) {
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
			name: "with envfile containing variables",
			repo: "envfile-repo",
			setupEnv: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "test-project")
				viper.Set(config.Branch, "main")
				testhelper.SetupDirs(t, ctx, []string{"envfile-repo"})

				// Create a temporary envfile
				tmpfile, err := os.CreateTemp("", ".env*")
				if err != nil {
					t.Fatalf("Failed to create temp envfile: %v", err)
				}
				t.Cleanup(func() { os.Remove(tmpfile.Name()) })

				envContent := `# Comment line
ENV_FILE_VAR1=value1
ENV_FILE_VAR2=value2

ENV_FILE_VAR3=value3`
				if _, err := tmpfile.WriteString(envContent); err != nil {
					t.Fatalf("Failed to write envfile: %v", err)
				}
				tmpfile.Close()

				viper.Set(config.CmdEnv, []string{tmpfile.Name()})
			},
			validate: func(t *testing.T, env []string) {
				testhelper.AssertContains(t, env, "ENV_FILE_VAR1=value1")
				testhelper.AssertContains(t, env, "ENV_FILE_VAR2=value2")
				testhelper.AssertContains(t, env, "ENV_FILE_VAR3=value3")
			},
		},
		{
			name: "with mixed key=value and envfile",
			repo: "mixed-repo",
			setupEnv: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "mixed-project")
				viper.Set(config.Branch, "mixed")
				testhelper.SetupDirs(t, ctx, []string{"mixed-repo"})

				tmpfile, err := os.CreateTemp("", ".env*")
				if err != nil {
					t.Fatalf("Failed to create temp envfile: %v", err)
				}
				t.Cleanup(func() { os.Remove(tmpfile.Name()) })

				if _, err := tmpfile.WriteString("FILE_VAR=from-file"); err != nil {
					t.Fatalf("Failed to write envfile: %v", err)
				}
				tmpfile.Close()

				viper.Set(config.CmdEnv, []string{"DIRECT_VAR=direct-value", tmpfile.Name()})
			},
			validate: func(t *testing.T, env []string) {
				testhelper.AssertContains(t, env, "DIRECT_VAR=direct-value")
				testhelper.AssertContains(t, env, "FILE_VAR=from-file")
			},
		},
		{
			name:         "with invalid envfile path",
			repo:         "invalid-repo",
			expectError:  true,
			errorMessage: "failed to read envfile",
			setupEnv: func(t *testing.T, ctx context.Context) {
				viper := config.Viper(ctx)
				viper.Set(config.GitProject, "test-project")
				viper.Set(config.Branch, "main")
				testhelper.SetupDirs(t, ctx, []string{"invalid-repo"})

				// Set a non-existent file path
				viper.Set(config.CmdEnv, []string{"/nonexistent/path/.env"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			if tt.setupEnv != nil {
				tt.setupEnv(t, ctx)
			}

			env, err := utils.Env(ctx, tt.repo)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorMessage != "" && !strings.Contains(err.Error(), tt.errorMessage) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMessage, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if env == nil {
				t.Fatal("Env returned nil")
			}

			if len(env) < 3 {
				t.Errorf("Env returned %d vars, expected at least 3", len(env))
			}

			if tt.validate != nil {
				tt.validate(t, env)
			}
		})
	}
}
