package auth

import (
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestFingerprint(t *testing.T) {
	if got := fingerprint(""); got != "" {
		t.Errorf("fingerprint(empty) = %q, want empty", got)
	}

	first := fingerprint("token-one")
	second := fingerprint("token-two")

	if first == second {
		t.Error("Different credentials must produce different fingerprints")
	}

	if first != fingerprint("token-one") {
		t.Error("fingerprint must be stable for the same credential")
	}

	// The digest must be one-way and short enough to be useless to an attacker.
	if !strings.HasPrefix(first, "sha256:") {
		t.Errorf("fingerprint %q should name its digest algorithm", first)
	}

	if digest := strings.TrimPrefix(first, "sha256:"); len(digest) != fingerprintLength {
		t.Errorf("digest %q length = %d, want %d", digest, len(digest), fingerprintLength)
	}

	if strings.Contains(first, "token-one") {
		t.Errorf("fingerprint %q must not contain the credential", first)
	}
}

func TestScopeSource(t *testing.T) {
	tests := []struct {
		name  string
		scope Scope
		want  string
	}{
		{
			name:  "env names the configured environment variable",
			scope: Scope{Provider: BackendEnv, Env: "ACME_TOKEN"},
			want:  "$ACME_TOKEN",
		},
		{
			name:  "env falls back to the default environment variable",
			scope: Scope{Provider: BackendEnv},
			want:  "$" + config.DefaultAuthEnv,
		},
		{
			name:  "command reports only the program name",
			scope: Scope{Provider: BackendCommand, Command: []string{"pass", "show", "acme/token"}},
			want:  "pass",
		},
		{
			name:  "command without configuration is called out",
			scope: Scope{Provider: BackendCommand},
			want:  "(auth.command not set)",
		},
		{
			name:  "gh names the selected account",
			scope: Scope{Provider: BackendGh, Account: "work-user"},
			want:  "gh (user: work-user)",
		},
		{
			name:  "gh without an account uses the active one",
			scope: Scope{Provider: BackendGh},
			want:  "gh (active account)",
		},
		{
			name:  "none is unauthenticated",
			scope: Scope{Provider: BackendNone},
			want:  "unauthenticated",
		},
		{
			name:  "auto is unknown until resolved",
			scope: Scope{Provider: BackendAuto},
			want:  "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scope.Source(); got != tt.want {
				t.Errorf("Source() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCommandSourceOmitsArguments matters because credential helper arguments
// can name secrets or vault paths which should not be echoed into a report.
func TestCommandSourceOmitsArguments(t *testing.T) {
	scope := Scope{Provider: BackendCommand, Command: []string{"vault", "read", "-field=token", "secret/acme"}}

	if got := scope.Source(); strings.Contains(got, "secret/acme") {
		t.Errorf("Source() = %q, should omit command arguments", got)
	}
}

func TestDescribe(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthOwners, map[string]any{
		"acme":    map[string]any{"env": "ACME_TOKEN"},
		"public":  map[string]any{"provider": BackendNone},
		"missing": map[string]any{"env": "ABSENT_TOKEN"},
	})

	t.Setenv("ACME_TOKEN", "acme-secret")

	statuses := DescribeAll(ctx, "github.com", "acme", "public", "missing")

	if len(statuses) != 3 {
		t.Fatalf("DescribeAll() returned %d statuses, want 3", len(statuses))
	}

	resolved := statuses[0]
	if !resolved.OK() {
		t.Errorf("acme should resolve, got error: %v", resolved.Err)
	}

	if resolved.Source != "$ACME_TOKEN" {
		t.Errorf("acme Source = %q, want %q", resolved.Source, "$ACME_TOKEN")
	}

	if resolved.Fingerprint == "" {
		t.Error("acme should report a fingerprint")
	}

	// A Status is rendered directly to the terminal, so it must never carry the
	// credential in any field.
	if strings.Contains(resolved.Source+resolved.Fingerprint+resolved.Provider, "acme-secret") {
		t.Error("Status must not contain the credential")
	}

	unauthenticated := statuses[1]
	if !unauthenticated.OK() {
		t.Errorf("public should be OK, got error: %v", unauthenticated.Err)
	}

	if !unauthenticated.Unauthenticated {
		t.Error("public should be marked unauthenticated")
	}

	if unauthenticated.Fingerprint != "" {
		t.Errorf("public should have no fingerprint, got %q", unauthenticated.Fingerprint)
	}

	failed := statuses[2]
	if failed.OK() {
		t.Error("missing should not resolve")
	}

	if failed.Fingerprint != "" {
		t.Errorf("missing should have no fingerprint, got %q", failed.Fingerprint)
	}
}

// TestDescribeDistinguishesOwners is the property that makes the fingerprint
// column worth showing: confirming two projects use different credentials.
func TestDescribeDistinguishesOwners(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthOwners, map[string]any{
		"personal-org": map[string]any{"env": "PERSONAL_TOKEN"},
		"work-org":     map[string]any{"env": "WORK_TOKEN"},
		"duplicate":    map[string]any{"env": "PERSONAL_TOKEN"},
	})

	t.Setenv("PERSONAL_TOKEN", "personal-secret")
	t.Setenv("WORK_TOKEN", "work-secret")

	statuses := DescribeAll(ctx, "github.com", "personal-org", "work-org", "duplicate")

	if statuses[0].Fingerprint == statuses[1].Fingerprint {
		t.Error("Projects with different credentials should have different fingerprints")
	}

	if statuses[0].Fingerprint != statuses[2].Fingerprint {
		t.Error("Projects sharing a credential should have matching fingerprints")
	}
}
