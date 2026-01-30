package call

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

// TestWrap tests the Wrap function which combines multiple CallFuncs
func TestWrap(t *testing.T) {
	ctx := loadFixture(t)

	testRepo := "test-repo"
	output1 := "output1 from " + testRepo
	output2 := "output2 from " + testRepo

	tests := []struct {
		name       string
		callFuncs  []Func
		repo       string
		wantOutput []string
		wantError  bool
	}{
		{
			name: "basic wrap with two functions",
			callFuncs: []Func{
				fakeCallFunc(t, false, output1),
				fakeCallFunc(t, false, output2),
			},
			repo:       testRepo,
			wantOutput: []string{output1, output2},
		},
		{
			name: "wrap with error in first function",
			callFuncs: []Func{
				fakeCallFunc(t, true, output1),
				fakeCallFunc(t, false, output2),
			},
			repo:       testRepo,
			wantOutput: []string{output1},
			wantError:  true,
		},
		{
			name: "wrap with error in second function",
			callFuncs: []Func{
				fakeCallFunc(t, false, output1),
				fakeCallFunc(t, true, output2),
			},
			repo:       testRepo,
			wantOutput: []string{output1, output2},
			wantError:  true,
		},
		{
			name:       "wrap with no CallFuncs",
			callFuncs:  []Func{},
			repo:       testRepo,
			wantOutput: []string{},
		},
		{
			name: "wrap with single function",
			callFuncs: []Func{
				fakeCallFunc(t, false, "test output"),
			},
			repo:       testRepo,
			wantOutput: []string{"test output"},
		},
		{
			name: "wrap repository cloning scenario",
			callFuncs: []Func{
				fakeCallFunc(t, false, "test after clone"),
			},
			repo:       "nonexistent-repo",
			wantOutput: []string{"test after clone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapper := Wrap(tt.callFuncs...)
			if wrapper == nil {
				t.Fatal("Wrap returned nil")
			}

			ch := output.NewChannel(ctx, tt.repo, nil, nil)
			err := wrapper(ctx, ch)
			ch.Close()

			// Collect output - reconstruct lines from byte chunks
			var buffer []byte
			for msg := range ch.Out() {
				buffer = append(buffer, msg...)
			}
			res := strings.Split(strings.TrimSuffix(string(buffer), "\n"), "\n")
			if len(buffer) == 0 {
				res = []string{}
			}

			testhelper.AssertOutput(t, res, tt.wantOutput, err, tt.wantError)
		})
	}
}

// TestExec tests the Exec function which creates CallFuncs for shell commands
func TestExec(t *testing.T) {
	ctx := loadFixture(t)

	testMessage := "test message"
	multilineOutput := "line1\nline2\nline3"

	tests := []struct {
		name       string
		command    string
		args       []string
		wantOutput []string
		wantError  bool
	}{
		{
			name:       "basic echo command",
			command:    "echo",
			args:       []string{testMessage},
			wantOutput: []string{testMessage},
		},
		{
			name:       "multiple arguments",
			command:    "echo",
			args:       []string{"hello", "world", "test"},
			wantOutput: []string{"hello world test"},
		},
		{
			name:      "nonexistent command",
			command:   "nonexistent-command-xyz",
			args:      []string{"arg1"},
			wantError: true,
		},
		{
			name:      "empty command",
			command:   "",
			args:      []string{"arg1"},
			wantError: true,
		},
		{
			name:       "multiline output",
			command:    "echo",
			args:       []string{multilineOutput},
			wantOutput: []string{"line1", "line2", "line3"},
		},
		{
			name:      "command with exit failure",
			command:   "false",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create repository directory for commands to run in
			repoPath := utils.RepoPath(ctx, "test-repo")
			if err := os.MkdirAll(repoPath, 0755); err != nil {
				t.Fatalf("Failed to create repo directory: %v", err)
			}
			t.Cleanup(func() {
				os.RemoveAll(repoPath)
			})

			execFunc := Exec(tt.command, tt.args...)
			if execFunc == nil {
				t.Fatal("Exec returned nil")
			}

			ch := output.NewChannel(ctx, "test-repo", nil, nil)
			err := execFunc(ctx, ch)
			ch.Close()

			// Collect output - reconstruct lines from byte chunks
			var buffer []byte
			for msg := range ch.Out() {
				buffer = append(buffer, msg...)
			}
			output := strings.Split(strings.TrimSuffix(string(buffer), "\n"), "\n")
			if len(buffer) == 0 {
				output = []string{}
			}

			testhelper.AssertOutput(t, output, tt.wantOutput, err, tt.wantError)
		})
	}
}

func TestExecIncludesMetadataEnv(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)
	viper.Set(config.GitProject, "test-project")
	viper.Set(config.Branch, "feature/test")
	viper.Set(config.DefaultBranch, "main")

	repo := "test-repo"
	testhelper.SetupDirs(t, ctx, []string{repo})

	execFunc := Exec("env")
	if execFunc == nil {
		t.Fatal("Exec returned nil")
	}

	ch := output.NewChannel(ctx, repo, nil, nil)
	err := execFunc(ctx, ch)
	ch.Close()

	if err != nil {
		t.Fatalf("Exec env failed: %v", err)
	}

	// Collect output - reconstruct lines from byte chunks
	var buffer []byte
	for msg := range ch.Out() {
		buffer = append(buffer, msg...)
	}
	outputLines := strings.Split(strings.TrimSuffix(string(buffer), "\n"), "\n")
	if len(buffer) == 0 {
		t.Fatal("expected env output, got empty output")
	}

	testhelper.AssertContains(t, outputLines, []string{
		"REPO_NAME=" + repo,
		"GIT_BRANCH=feature/test",
		"GIT_DEFAULT_BRANCH=main",
		"GIT_PROJECT=test-project",
	})
}

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

			env, err := Env(ctx, tt.repo)

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
