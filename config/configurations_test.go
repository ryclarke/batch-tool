package config_test

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
)

// TestGitUpdateStashPrecedence covers the grouped git.update.stash key, which is
// nested two levels deep in the config file, and confirms an explicit flag still
// wins over a config file value.
func TestGitUpdateStashPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		setFlag  bool
		expected bool
	}{
		{
			name:     "unset falls back to the default",
			yaml:     "git: {}\n",
			expected: false,
		},
		{
			name:     "nested config key is honored",
			yaml:     "git:\n  update:\n    stash: true\n",
			expected: true,
		},
		{
			name:     "explicit flag overrides the config key",
			yaml:     "git:\n  update:\n    stash: true\n",
			setFlag:  true,
			expected: false,
		},
		{
			name:     "the removed git.stash-updates key is ignored",
			yaml:     "git:\n  stash-updates: true\n",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := config.New()

			v.SetConfigType("yaml")
			if err := v.ReadConfig(strings.NewReader(tt.yaml)); err != nil {
				t.Fatalf("Failed reading test config: %v", err)
			}

			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			flags.Bool("stash", false, "")

			if tt.setFlag {
				if err := flags.Set("stash", "false"); err != nil {
					t.Fatalf("Failed setting stash flag: %v", err)
				}
			}

			if err := v.BindPFlag(config.GitUpdateStash, flags.Lookup("stash")); err != nil {
				t.Fatalf("Failed binding stash flag: %v", err)
			}

			if got := v.GetBool(config.GitUpdateStash); got != tt.expected {
				t.Errorf("Expected %s to be %v, got %v", config.GitUpdateStash, tt.expected, got)
			}
		})
	}
}

// TestGroupedGitDefaults verifies the per-subcommand config groups all resolve to
// the behavior they replaced.
func TestGroupedGitDefaults(t *testing.T) {
	v := config.New()

	strs := map[string]string{
		config.GitRemote:             "origin",
		config.GitCommitStage:        "all",
		config.GitUpdatePullStrategy: "default",
		config.GitStashScope:         "all",
	}

	for key, expected := range strs {
		if got := v.GetString(key); got != expected {
			t.Errorf("Expected %s to default to %q, got %q", key, expected, got)
		}
	}

	bools := map[string]bool{
		config.GitCommitPush:         false,
		config.GitUpdateStash:        false,
		config.GitUpdateCleanIgnored: false,
		config.GitUpdateSubmodules:   true,
		config.GitBranchReset:        true,
	}

	for key, expected := range bools {
		if got := v.GetBool(key); got != expected {
			t.Errorf("Expected %s to default to %v, got %v", key, expected, got)
		}
	}
}

func TestSetViperAndViper(t *testing.T) {
	v := viper.New()
	ctx := config.SetViper(context.Background(), v)

	if got := config.Viper(ctx); got != v {
		t.Errorf("Viper(ctx) returned a different instance than the one stored")
	}
}

func TestSetViperWithNilFallsBackToGlobal(t *testing.T) {
	ctx := config.SetViper(context.Background(), nil)

	if got := config.Viper(ctx); got != viper.GetViper() {
		t.Errorf("expected nil viper to fall back to the global viper instance")
	}
}

func TestViperWithoutSetFallsBackToGlobal(t *testing.T) {
	if got := config.Viper(context.Background()); got != viper.GetViper() {
		t.Errorf("expected empty context to fall back to the global viper instance")
	}
}

func TestSetChildIsolation(t *testing.T) {
	parent := viper.New()
	parent.Set("foo", "parent-value")

	ctx := config.SetViper(context.Background(), parent)
	childCtx := config.SetChild(ctx)

	child := config.Viper(childCtx)
	if child == parent {
		t.Fatal("SetChild should produce a distinct viper instance")
	}

	// Child should mirror parent's settings at construction time.
	if got := child.GetString("foo"); got != "parent-value" {
		t.Errorf("child viper missing parent setting: got %q", got)
	}

	// Mutations on the child must not leak to the parent.
	child.Set("foo", "child-value")
	if got := parent.GetString("foo"); got != "parent-value" {
		t.Errorf("child mutation leaked to parent: got %q", got)
	}
}

// TestWithCancelAndCancel verifies that the cancel function attached by config.WithCancel
// can be retrieved via config.Cancel and propagates context cancellation.
func TestWithCancelAndCancel(t *testing.T) {
	ctx := context.Background()

	cancelCtx, cancel := config.WithCancel(ctx)
	defer cancel()

	// Cancel via the context value (mirrors what an output handler does on quit).
	config.Cancel(cancelCtx)()

	select {
	case <-cancelCtx.Done():
		// expected
	default:
		t.Fatal("expected context to be cancelled after Cancel(ctx)()")
	}
}

// TestCancelMissingReturnsNoOp verifies that Cancel on a context without an
// attached cancel function returns a safe no-op (does not panic).
func TestCancelMissingReturnsNoOp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Cancel on plain context panicked: %v", r)
		}
	}()

	config.Cancel(context.Background())()
}
