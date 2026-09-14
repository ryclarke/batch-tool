package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestGhAvailable(t *testing.T) {
	stubLookPath(t, true)

	if !ghAvailable() {
		t.Error("ghAvailable() = false, want true when gh is on PATH")
	}

	stubLookPath(t, false)

	if ghAvailable() {
		t.Error("ghAvailable() = true, want false when gh is absent")
	}
}

func TestGhTokenArguments(t *testing.T) {
	tests := []struct {
		name  string
		host  string
		scope Scope
		want  []string
	}{
		{
			name: "host only",
			host: "github.com",
			want: []string{"auth", "token", "--hostname", "github.com"},
		},
		{
			name:  "host and account",
			host:  "github.com",
			scope: Scope{Account: "work-user"},
			want:  []string{"auth", "token", "--hostname", "github.com", "--user", "work-user"},
		},
		{
			name:  "enterprise host",
			host:  "github.example.com",
			scope: Scope{Account: "monalisa"},
			want:  []string{"auth", "token", "--hostname", "github.example.com", "--user", "monalisa"},
		},
		{
			name: "no host falls back to the gh default",
			want: []string{"auth", "token"},
		},
		{
			name: "blank host is ignored",
			host: "   ",
			want: []string{"auth", "token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotName string
			var gotArgs []string

			stubRunCmd(t, func(_ context.Context, name string, args ...string) ([]byte, error) {
				gotName, gotArgs = name, args

				return []byte("gh-token"), nil
			})

			token, err := ghToken(context.Background(), tt.host, tt.scope)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if token != "gh-token" {
				t.Errorf("token = %q, want %q", token, "gh-token")
			}

			if gotName != ghCommand {
				t.Errorf("command = %q, want %q", gotName, ghCommand)
			}

			if !equalSlices(gotArgs, tt.want) {
				t.Errorf("args = %v, want %v", gotArgs, tt.want)
			}
		})
	}
}

func TestGhTokenEmptyOutput(t *testing.T) {
	stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
		return []byte("\n"), nil
	})

	_, err := ghToken(context.Background(), "github.com", Scope{Owner: "acme"})
	if !errors.Is(err, ErrNoCredential) {
		t.Errorf("error = %v, want ErrNoCredential", err)
	}

	if !strings.Contains(err.Error(), "gh auth login") {
		t.Errorf("error %q should suggest `gh auth login`", err)
	}
}

// TestGhTokenErrorExcludesStdout guards the redaction boundary: gh writes the
// token to stdout, so only stderr may ever appear in an error.
func TestGhTokenErrorExcludesStdout(t *testing.T) {
	const secret = "gho_supersecretvalue"

	stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
		return []byte(secret), errors.New("exit status 1")
	})

	_, err := ghToken(context.Background(), "github.com", Scope{Owner: "acme"})
	if err == nil {
		t.Fatal("Expected an error")
	}

	if strings.Contains(err.Error(), secret) {
		t.Errorf("error %q must not contain the credential", err)
	}
}

func TestGhTokenTrimsOutput(t *testing.T) {
	stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
		return []byte("  gho_padded\n\n"), nil
	})

	token, err := ghToken(context.Background(), "github.com", Scope{})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if token != "gho_padded" {
		t.Errorf("token = %q, want %q", token, "gho_padded")
	}
}

func TestFormatStderr(t *testing.T) {
	// A non-exec error carries no stderr to report.
	if got := formatStderr(errors.New("plain")); got != "" {
		t.Errorf("formatStderr(plain error) = %q, want empty", got)
	}
}

// TestGhBackendPerAccount covers the multi-account case which motivates the
// gh backend: two projects on one host authenticating as different gh accounts.
func TestGhBackendPerAccount(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProvider, githubProvider)
	viper.Set(config.AuthProvider, BackendGh)
	viper.Set(config.AuthOwners, map[string]any{
		"personal-org": map[string]any{"account": "personal-user"},
		"work-org":     map[string]any{"account": "work-user"},
	})

	stubLookPath(t, true)
	stubRunCmd(t, func(_ context.Context, _ string, args ...string) ([]byte, error) {
		// Return a distinct token per requested account.
		for i, arg := range args {
			if arg == "--user" && i+1 < len(args) {
				return []byte("token-for-" + args[i+1]), nil
			}
		}

		return nil, errors.New("no account requested")
	})

	requireToken(t, ctx, "github.com", "personal-org", "token-for-personal-user")
	requireToken(t, ctx, "github.com", "work-org", "token-for-work-user")
}
