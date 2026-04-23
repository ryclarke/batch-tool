package config

import (
	"context"

	"github.com/spf13/viper"
)

type contextKey struct{ key string }

var configKey = &contextKey{"viper"}

type cancelKey struct{}

// SetViper saves the Viper instance into the context.
func SetViper(ctx context.Context, v *viper.Viper) context.Context {
	if v == nil {
		// fallback to global viper instance
		v = viper.GetViper()
	}

	return context.WithValue(ctx, configKey, v)
}

// SetChild creates a child Viper instance and saves it into the context.
func SetChild(ctx context.Context) context.Context {
	v := Child(ctx)

	return SetViper(ctx, v)
}

// Viper retrieves the Viper instance from the context.
func Viper(ctx context.Context) *viper.Viper {
	v := ctx.Value(configKey)
	if v == nil {
		// fallback to global viper instance
		return viper.GetViper()
	}

	return v.(*viper.Viper)
}

// WithCancel returns a copy of ctx with a new cancel function attached as a
// context value. The returned cancel function should be deferred by the caller.
// Other packages can retrieve the same cancel function via Cancel(ctx).
func WithCancel(ctx context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)
	ctx = context.WithValue(ctx, cancelKey{}, cancel)

	return ctx, cancel
}

// Cancel returns the cancel function attached to ctx by WithCancel, or a no-op
// if none is present.
func Cancel(ctx context.Context) context.CancelFunc {
	if cancel, ok := ctx.Value(cancelKey{}).(context.CancelFunc); ok {
		return cancel
	}

	return func() { /* noop */ }
}
