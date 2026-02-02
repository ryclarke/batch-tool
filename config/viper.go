package config

import (
	"context"
	"strings"

	"github.com/spf13/viper"
)

// New creates a new Viper instance with default configuration.
func New() *viper.Viper {
	return newViper()
}

// Child creates a new Viper instance that inherits all settings from the parent context.
func Child(ctx context.Context) *viper.Viper {
	v := newViper()

	// Copy all settings from parent Viper
	for key, value := range Viper(ctx).AllSettings() {
		v.Set(key, value)
	}

	return v
}

func newViper() *viper.Viper {
	v := viper.NewWithOptions(viper.EnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_")))
	v.AutomaticEnv() // read in environment variables that match

	// Initialize default settings
	setDefaults(v)

	return v
}
