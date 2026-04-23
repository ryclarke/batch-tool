package config_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"

	"github.com/ryclarke/batch-tool/config"
)

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
