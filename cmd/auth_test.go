package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/auth"
	"github.com/ryclarke/batch-tool/config"
)

// runAuthStatus executes the auth status command against the given context and
// returns its combined output.
func runAuthStatus(t *testing.T, ctx context.Context, args ...string) (string, error) {
	t.Helper()

	cmd := authStatusCmd()
	cmd.SetContext(ctx)
	cmd.SilenceUsage = true

	var out bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return out.String(), err
}

func TestAuthCmd(t *testing.T) {
	loadFixture(t)

	cmd := authCmd()

	if cmd.Use != "auth" {
		t.Errorf("Expected Use to be 'auth', got %s", cmd.Use)
	}

	var found bool

	for _, sub := range cmd.Commands() {
		if sub.Name() == "status" {
			found = true
		}
	}

	if !found {
		t.Error("Expected auth command to have a status subcommand")
	}
}

func TestAuthCmdRegistered(t *testing.T) {
	loadFixture(t)

	for _, sub := range RootCmd().Commands() {
		if sub.Name() == "auth" {
			return
		}
	}

	t.Error("Expected auth command to be registered on the root command")
}

func TestAuthStatusReportsEachProject(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProject, "personal-org")
	viper.Set(config.GitProjects, []string{"work-org"})
	viper.Set(config.AuthProvider, auth.BackendEnv)
	viper.Set(config.AuthOwners, map[string]any{
		"personal-org": map[string]any{"env": "PERSONAL_TOKEN"},
		"work-org":     map[string]any{"env": "WORK_TOKEN"},
	})

	t.Setenv("PERSONAL_TOKEN", "personal-secret")
	t.Setenv("WORK_TOKEN", "work-secret")
	auth.Reset()

	out, err := runAuthStatus(t, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v\n%s", err, out)
	}

	for _, want := range []string{"personal-org", "work-org", "$PERSONAL_TOKEN", "$WORK_TOKEN", "ok"} {
		if !strings.Contains(out, want) {
			t.Errorf("Expected output to contain %q, got:\n%s", want, out)
		}
	}
}

// TestAuthStatusNeverPrintsCredentials is the central guarantee of this command:
// it reports on credentials without ever displaying one.
func TestAuthStatusNeverPrintsCredentials(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	const secret = "super-secret-token-value"

	viper.Set(config.GitProject, "acme")
	viper.Set(config.AuthProvider, auth.BackendEnv)
	viper.Set(config.AuthEnv, "ACME_TOKEN")

	t.Setenv("ACME_TOKEN", secret)
	auth.Reset()

	for _, args := range [][]string{nil, {"--verbose"}} {
		out, err := runAuthStatus(t, ctx, args...)
		if err != nil {
			t.Fatalf("Unexpected error: %v\n%s", err, out)
		}

		if strings.Contains(out, secret) {
			t.Errorf("Output for args %v must not contain the credential:\n%s", args, out)
		}
	}
}

func TestAuthStatusVerboseAddsDigest(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProject, "acme")
	viper.Set(config.AuthProvider, auth.BackendEnv)
	viper.Set(config.AuthEnv, "ACME_TOKEN")

	t.Setenv("ACME_TOKEN", "acme-secret")
	auth.Reset()

	plain, err := runAuthStatus(t, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Without -v the report contains nothing derived from the credential at all.
	if strings.Contains(plain, "sha256:") {
		t.Errorf("Default output should omit the digest, got:\n%s", plain)
	}

	verbose, err := runAuthStatus(t, ctx, "--verbose")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(verbose, "sha256:") {
		t.Errorf("Verbose output should include the digest, got:\n%s", verbose)
	}
}

func TestAuthStatusReportsUnauthenticated(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProject, "public-org")
	viper.Set(config.AuthProvider, auth.BackendNone)
	auth.Reset()

	out, err := runAuthStatus(t, ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v\n%s", err, out)
	}

	if !strings.Contains(out, "none required") {
		t.Errorf("Expected unauthenticated status, got:\n%s", out)
	}
}

func TestAuthStatusFailsWhenUnresolved(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProject, "acme")
	viper.Set(config.AuthProvider, auth.BackendEnv)
	viper.Set(config.AuthEnv, "ABSENT_TOKEN")
	auth.Reset()

	out, err := runAuthStatus(t, ctx)
	if err == nil {
		t.Fatalf("Expected an error when no credential resolves, got:\n%s", out)
	}

	if !strings.Contains(out, "FAILED") {
		t.Errorf("Expected output to mark the project FAILED, got:\n%s", out)
	}

	// The reason must be reported so the failure is actionable.
	if !strings.Contains(out, "ABSENT_TOKEN") {
		t.Errorf("Expected output to name the missing variable, got:\n%s", out)
	}
}

func TestAuthStatusRequiresConfiguredProject(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	viper.Set(config.GitProject, "")
	viper.Set(config.GitProjects, []string{})
	viper.Set(config.AuthOwners, map[string]any{})
	auth.Reset()

	if _, err := runAuthStatus(t, ctx); err == nil {
		t.Error("Expected an error when no projects are configured")
	}
}

func TestAuthStatusRejectsArguments(t *testing.T) {
	ctx := loadFixture(t)

	cmd := authStatusCmd()
	cmd.SetContext(ctx)

	if err := cmd.Args(cmd, []string{"unexpected"}); err == nil {
		t.Error("Expected auth status to reject positional arguments")
	}
}
