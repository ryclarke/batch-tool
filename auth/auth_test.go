package auth

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestResolveScopeDefaults(t *testing.T) {
	ctx := testContext(t)

	scope := ResolveScope(ctx, "acme")

	if scope.Owner != "acme" {
		t.Errorf("Owner = %q, want %q", scope.Owner, "acme")
	}

	if scope.Provider != BackendAuto {
		t.Errorf("Provider = %q, want %q", scope.Provider, BackendAuto)
	}

	if scope.Env != config.DefaultAuthEnv {
		t.Errorf("Env = %q, want %q", scope.Env, config.DefaultAuthEnv)
	}
}

func TestResolveScopeOwnerOverride(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthEnv, "DEFAULT_TOKEN")
	viper.Set(config.AuthAccount, "default-account")
	viper.Set(config.AuthOwners, map[string]any{
		"acme": map[string]any{
			"provider": BackendGh,
			"account":  "acme-account",
		},
	})

	tests := []struct {
		name         string
		owner        string
		wantProvider string
		wantEnv      string
		wantAccount  string
	}{
		{
			name:         "owner entry overrides only the fields it sets",
			owner:        "acme",
			wantProvider: BackendGh,
			wantEnv:      "DEFAULT_TOKEN",
			wantAccount:  "acme-account",
		},
		{
			name:         "unlisted owner falls through to the default scope",
			owner:        "other",
			wantProvider: BackendEnv,
			wantEnv:      "DEFAULT_TOKEN",
			wantAccount:  "default-account",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scope := ResolveScope(ctx, tt.owner)

			if scope.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", scope.Provider, tt.wantProvider)
			}

			if scope.Env != tt.wantEnv {
				t.Errorf("Env = %q, want %q", scope.Env, tt.wantEnv)
			}

			if scope.Account != tt.wantAccount {
				t.Errorf("Account = %q, want %q", scope.Account, tt.wantAccount)
			}
		})
	}
}

func TestResolveScopeOwnerCaseInsensitive(t *testing.T) {
	ctx := testContext(t)

	config.Viper(ctx).Set(config.AuthOwners, map[string]any{
		"Mixed-Case-Org": map[string]any{"env": "MIXED_CASE_TOKEN"},
	})

	// Owner names are case-insensitive on every supported provider, and viper
	// lowercases config keys, so lookups must match regardless of casing.
	for _, owner := range []string{"mixed-case-org", "Mixed-Case-Org", "MIXED-CASE-ORG"} {
		if got := ResolveScope(ctx, owner).Env; got != "MIXED_CASE_TOKEN" {
			t.Errorf("ResolveScope(%q).Env = %q, want %q", owner, got, "MIXED_CASE_TOKEN")
		}
	}
}

func TestResolveScopeOwnerCommand(t *testing.T) {
	ctx := testContext(t)

	config.Viper(ctx).Set(config.AuthOwners, map[string]any{
		"acme": map[string]any{
			"provider": BackendCommand,
			"command":  []any{"pass", "show", "acme/token"},
		},
	})

	scope := ResolveScope(ctx, "acme")

	want := []string{"pass", "show", "acme/token"}
	if !reflect.DeepEqual(scope.Command, want) {
		t.Errorf("Command = %v, want %v", scope.Command, want)
	}
}

func TestOwners(t *testing.T) {
	ctx := testContext(t)

	config.Viper(ctx).Set(config.AuthOwners, map[string]any{
		"acme":  map[string]any{"provider": BackendNone},
		"other": map[string]any{"provider": BackendNone},
	})

	got := Sorted(Owners(ctx)...)

	want := []string{"acme", "other"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Owners() = %v, want %v", got, want)
	}
}

func TestTokenCachedPerOwner(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthOwners, map[string]any{
		"acme":  map[string]any{"env": "ACME_TOKEN"},
		"other": map[string]any{"env": "OTHER_TOKEN"},
	})

	t.Setenv("ACME_TOKEN", "acme-secret")
	t.Setenv("OTHER_TOKEN", "other-secret")

	// Each owner resolves independently - this is the multi-organization case.
	requireToken(t, ctx, "github.com", "acme", "acme-secret")
	requireToken(t, ctx, "github.com", "other", "other-secret")

	// A cached credential survives the environment variable being cleared.
	t.Setenv("ACME_TOKEN", "")
	requireToken(t, ctx, "github.com", "acme", "acme-secret")
}

func TestTokenCacheKeyedByHost(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthEnv, "SHARED_TOKEN")

	t.Setenv("SHARED_TOKEN", "first")
	requireToken(t, ctx, "github.com", "acme", "first")

	// A different host is a separate cache entry, so it resolves afresh.
	t.Setenv("SHARED_TOKEN", "second")
	requireToken(t, ctx, "github.example.com", "acme", "second")

	// The original host keeps its cached value.
	requireToken(t, ctx, "github.com", "acme", "first")
}

func TestTokenDeduplicatesConcurrentLookups(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendCommand)
	viper.Set(config.AuthCommand, []string{"credential-helper"})

	var calls atomic.Int64

	stubRunCmd(t, func(context.Context, string, ...string) ([]byte, error) {
		calls.Add(1)

		return []byte("shared-secret\n"), nil
	})

	// call.Do fans out one goroutine per repository, so a cold cache must not
	// spawn one subprocess per caller.
	const goroutines = 25

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for range goroutines {
		go func() {
			defer wg.Done()

			if token, err := Token(ctx, "github.com", "acme"); err != nil || token != "shared-secret" {
				t.Errorf("Token() = %q, %v; want %q, nil", token, err, "shared-secret")
			}
		}()
	}

	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("credential helper ran %d times, want 1", got)
	}
}

func TestValidate(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthEnv, "VALIDATE_TOKEN")

	if err := Validate(ctx, "github.com", "acme"); err == nil {
		t.Error("Validate() with no credential should return an error")
	}

	t.Setenv("VALIDATE_TOKEN", "secret")
	Reset()

	if err := Validate(ctx, "github.com", "acme"); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}
}

func TestResetClearsCache(t *testing.T) {
	ctx := testContext(t)
	viper := config.Viper(ctx)

	viper.Set(config.AuthProvider, BackendEnv)
	viper.Set(config.AuthEnv, "RESET_TOKEN")

	t.Setenv("RESET_TOKEN", "before")
	requireToken(t, ctx, "github.com", "acme", "before")

	t.Setenv("RESET_TOKEN", "after")
	Reset()

	requireToken(t, ctx, "github.com", "acme", "after")
}

func TestSorted(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "sorts and deduplicates",
			input: []string{"other", "acme", "other"},
			want:  []string{"acme", "other"},
		},
		{
			name:  "deduplicates case-insensitively keeping the first spelling",
			input: []string{"Acme", "acme", "ACME"},
			want:  []string{"Acme"},
		},
		{
			name:  "drops empty and whitespace-only names",
			input: []string{"", "  ", "acme"},
			want:  []string{"acme"},
		},
		{
			name:  "handles no input",
			input: nil,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Sorted(tt.input...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Sorted(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
