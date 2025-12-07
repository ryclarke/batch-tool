package config

import (
	"context"

	"github.com/spf13/viper"
)

type contextKey struct{ key string }

var configKey = &contextKey{"viper"}

// SetViper saves the Viper instance into the context.
func SetViper(ctx context.Context, v *viper.Viper) context.Context {
	if v == nil {
		// fallback to global viper instance
		v = viper.GetViper()
	}

	return context.WithValue(ctx, configKey, v)
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
