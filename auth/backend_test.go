package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestBackendNone(t *testing.T) {
	ctx := testContext(t)

	config.Viper(ctx).Set(config.AuthProvider, BackendNone)

	token, err := Token(ctx, "github.com", "acme")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if token != "" {
		t.Errorf("Token() = %q, want an empty token for unauthenticated access", token)
	}
}

func TestBackendEnvMissing(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthEnv, "MISSING_TOKEN")

	err := requireTokenError(t, ctx, "github.com")

	if !errors.Is(err, ErrNoCredential) {
		t.Errorf("error = %v, want ErrNoCredential", err)
	}

	// The message must name the variable so the user knows what to set.
	if !strings.Contains(err.Error(), "MISSING_TOKEN") {
		t.Errorf("error %q should name the missing environment variable", err)
	}
}

func TestBackendEnvTrimsWhitespace(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthEnv, "PADDED_TOKEN")

	t.Setenv("PADDED_TOKEN", "  secret\n")

	requireToken(t, ctx, "github.com", "acme", "secret")
}

func TestBackendUnknown(t *testing.T) {
	ctx := testContext(t)

	config.Viper(ctx).Set(config.AuthProvider, "telepathy")

	err := requireTokenError(t, ctx, "github.com")

	// The message should list the valid choices.
	for _, backend := range Backends {
		if !strings.Contains(err.Error(), backend) {
			t.Errorf("error %q should list the %q backend", err, backend)
		}
	}
}

func TestBackendCommand(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendCommand)
	viper.Set(config.AuthCommand, []string{"credential-helper", "--owner", "acme"})

	var gotName string
	var gotArgs []string

	stubRunCmd(t, func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args

		return []byte("command-secret\n"), nil
	})

	requireToken(t, ctx, "github.com", "acme", "command-secret")

	// argv is passed through directly, never assembled into a shell string.
	if gotName != "credential-helper" {
		t.Errorf("command name = %q, want %q", gotName, "credential-helper")
	}

	if want := []string{"--owner", "acme"}; !equalSlices(gotArgs, want) {
		t.Errorf("command args = %v, want %v", gotArgs, want)
	}
}

func TestBackendCommandNotConfigured(t *testing.T) {
	ctx := testContext(t)

	config.Viper(ctx).Set(config.AuthProvider, BackendCommand)

	err := requireTokenError(t, ctx, "github.com")

	if !strings.Contains(err.Error(), config.AuthCommand) {
		t.Errorf("error %q should name the %s setting", err, config.AuthCommand)
	}
}

func TestBackendCommandEmptyOutput(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendCommand)
	viper.Set(config.AuthCommand, []string{"credential-helper"})

	stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
		return []byte("  \n"), nil
	})

	if err := requireTokenError(t, ctx, "github.com"); !errors.Is(err, ErrNoCredential) {
		t.Errorf("error = %v, want ErrNoCredential", err)
	}
}

// TestBackendCommandErrorExcludesStdout is the redaction guarantee: a failing
// credential helper may already have written the secret to stdout, which must
// never reach an error message or a log.
func TestBackendCommandErrorExcludesStdout(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendCommand)
	viper.Set(config.AuthCommand, []string{"credential-helper"})

	const secret = "super-secret-token-value"

	stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
		return []byte(secret), fmt.Errorf("helper exploded")
	})

	err := requireTokenError(t, ctx, "github.com")

	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q must not contain the credential", err)
	}

	if !strings.Contains(err.Error(), "credential-helper") {
		t.Errorf("error %q should name the failing command", err)
	}
}

// TestBackendCommandErrorIncludesStderr uses a real subprocess so the
// exec.ExitError stderr capture is exercised end to end.
func TestBackendCommandErrorIncludesStderr(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendCommand)
	viper.Set(config.AuthCommand, []string{"sh", "-c", "echo vault is sealed >&2; exit 3"})

	err := requireTokenError(t, ctx, "github.com")

	if !strings.Contains(err.Error(), "vault is sealed") {
		t.Errorf("error %q should include the command's stderr", err)
	}
}

func TestBackendGhRequiresGithubProvider(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProvider, "bitbucket")
	viper.Set(config.AuthProvider, BackendGh)

	stubLookPath(t, true)

	err := requireTokenError(t, ctx, "bitbucket.example.com")

	if !strings.Contains(err.Error(), githubProvider) {
		t.Errorf("error %q should explain that the gh backend is GitHub-only", err)
	}
}

func TestBackendGhRequiresGhInstalled(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProvider, githubProvider)
	viper.Set(config.AuthProvider, BackendGh)

	stubLookPath(t, false)

	err := requireTokenError(t, ctx, "github.com")

	// An explicit gh backend must fail loudly and actionably when gh is absent.
	if !strings.Contains(err.Error(), "gh") || !strings.Contains(err.Error(), "cli.github.com") {
		t.Errorf("error %q should tell the user how to install gh", err)
	}
}

func TestAutoChainOrder(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		env        map[string]string
		authToken  string
		ghToken    string
		ghPresent  bool
		wantToken  string
		wantSource string
	}{
		{
			name:       "prefers the configured environment variable",
			provider:   githubProvider,
			env:        map[string]string{config.DefaultAuthEnv: "from-auth-env", "GH_TOKEN": "from-gh-env"},
			ghToken:    "from-gh-cli",
			ghPresent:  true,
			wantToken:  "from-auth-env",
			wantSource: "$" + config.DefaultAuthEnv,
		},
		{
			// An explicitly configured credential must never be silently replaced
			// by a discovered one, which would change behavior for existing users
			// the moment they install the gh CLI.
			name:       "prefers the deprecated setting over discovered sources",
			provider:   githubProvider,
			authToken:  "from-deprecated-setting",
			env:        map[string]string{"GH_TOKEN": "from-gh-env"},
			ghToken:    "from-gh-cli",
			ghPresent:  true,
			wantToken:  "from-deprecated-setting",
			wantSource: config.AuthToken + " (deprecated)",
		},
		{
			name:       "falls back to GH_TOKEN for GitHub Actions",
			provider:   githubProvider,
			env:        map[string]string{"GH_TOKEN": "from-gh-env"},
			ghToken:    "from-gh-cli",
			ghPresent:  true,
			wantToken:  "from-gh-env",
			wantSource: "$GH_TOKEN",
		},
		{
			name:       "falls back to GITHUB_TOKEN for GitHub Actions",
			provider:   githubProvider,
			env:        map[string]string{"GITHUB_TOKEN": "from-github-env"},
			ghToken:    "from-gh-cli",
			ghPresent:  true,
			wantToken:  "from-github-env",
			wantSource: "$GITHUB_TOKEN",
		},
		{
			name:       "falls back to the gh CLI when no environment variable is set",
			provider:   githubProvider,
			ghToken:    "from-gh-cli",
			ghPresent:  true,
			wantToken:  "from-gh-cli",
			wantSource: "gh (active account)",
		},
		{
			name:       "skips the gh CLI when it is not installed",
			provider:   githubProvider,
			authToken:  "from-deprecated-setting",
			ghPresent:  false,
			wantToken:  "from-deprecated-setting",
			wantSource: config.AuthToken + " (deprecated)",
		},
		{
			name:       "ignores GitHub sources for other providers",
			provider:   "bitbucket",
			env:        map[string]string{"GH_TOKEN": "from-gh-env"},
			authToken:  "from-deprecated-setting",
			ghPresent:  true,
			wantToken:  "from-deprecated-setting",
			wantSource: config.AuthToken + " (deprecated)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testContext(t)
			viper := config.Viper(ctx)

			viper.Set(config.GitProvider, tt.provider)
			viper.Set(config.AuthProvider, BackendAuto)

			if tt.authToken != "" {
				viper.Set(config.AuthToken, tt.authToken)
			}

			for name, value := range tt.env {
				t.Setenv(name, value)
			}

			stubLookPath(t, tt.ghPresent)
			stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
				if tt.ghToken == "" {
					return nil, errors.New("not logged in")
				}

				return []byte(tt.ghToken), nil
			})

			requireToken(t, ctx, "github.com", "acme", tt.wantToken)

			if got := Describe(ctx, "github.com", "acme").Source; got != tt.wantSource {
				t.Errorf("Source = %q, want %q", got, tt.wantSource)
			}
		})
	}
}

func TestAutoChainExhausted(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProvider, githubProvider)
	viper.Set(config.AuthProvider, BackendAuto)

	stubLookPath(t, false)

	err := requireTokenError(t, ctx, "github.com")

	if !errors.Is(err, ErrNoCredential) {
		t.Errorf("error = %v, want ErrNoCredential", err)
	}

	// The message should name the owner so multi-project failures are attributable.
	if !strings.Contains(err.Error(), "acme") {
		t.Errorf("error %q should name the project", err)
	}
}

// TestAutoChainTolerantOfGhFailure confirms the gh CLI stays optional: a gh
// failure falls through to the remaining sources instead of aborting.
func TestAutoChainTolerantOfGhFailure(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProvider, githubProvider)
	viper.Set(config.AuthProvider, BackendAuto)
	viper.Set(config.AuthToken, "from-deprecated-setting")

	stubLookPath(t, true)
	stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("gh not logged in")
	})

	requireToken(t, ctx, "github.com", "acme", "from-deprecated-setting")
}

func TestDescribeOwner(t *testing.T) {
	if got := describeOwner(Scope{}); got != "the default project" {
		t.Errorf("describeOwner(empty) = %q, want %q", got, "the default project")
	}

	if got := describeOwner(Scope{Owner: "acme"}); got != "project acme" {
		t.Errorf("describeOwner(acme) = %q, want %q", got, "project acme")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
