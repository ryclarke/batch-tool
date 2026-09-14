package auth

import (
	"context"
	"os/exec"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

// These tests build their viper context directly rather than using the shared
// utils/testing fixtures, because that package imports auth and would introduce
// an import cycle into the test binary.

// testOwner is the project used by tests which only need a single owner.
const testOwner = "acme"

// testContext returns a context with default configuration, a cleared credential
// cache, and no ambient credentials in the environment.
func testContext(t *testing.T) context.Context {
	t.Helper()

	Reset()
	t.Cleanup(Reset)

	// Clear every environment variable the resolver consults so tests never
	// inherit a credential from the developer's shell or from CI.
	for _, env := range append([]string{config.DefaultAuthEnv}, githubEnvVars...) {
		t.Setenv(env, "")
	}

	return config.SetViper(context.Background(), config.New())
}

// stubRunCmd replaces the command runner for the duration of the test.
func stubRunCmd(t *testing.T, fn func(ctx context.Context, name string, args ...string) ([]byte, error)) {
	t.Helper()

	previous := runCmd
	runCmd = fn

	t.Cleanup(func() { runCmd = previous })
}

// stubLookPath controls whether the gh CLI appears to be installed.
func stubLookPath(t *testing.T, available bool) {
	t.Helper()

	previous := lookPath
	lookPath = func(file string) (string, error) {
		if available {
			return "/usr/bin/" + file, nil
		}

		return "", exec.ErrNotFound
	}

	t.Cleanup(func() { lookPath = previous })
}

// requireToken fails the test unless the expected token is resolved.
func requireToken(t *testing.T, ctx context.Context, host, owner, want string) {
	t.Helper()

	got, err := Token(ctx, host, owner)
	if err != nil {
		t.Fatalf("Token(%q, %q) unexpected error: %v", host, owner, err)
	}

	if got != want {
		t.Errorf("Token(%q, %q) = %q, want %q", host, owner, got, want)
	}
}

// requireTokenError fails the test unless resolution fails for testOwner.
func requireTokenError(t *testing.T, ctx context.Context, host string) error {
	t.Helper()

	token, err := Token(ctx, host, testOwner)
	if err == nil {
		t.Fatalf("Token(%q, %q) = %q, want an error", host, testOwner, token)
	}

	return err
}
