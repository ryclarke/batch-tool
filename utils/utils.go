package utils

import (
	"context"
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/cobra"
)

// CleanFilter standardizes the formatting of an imput argument by removing all configured signal tokens
func CleanFilter(ctx context.Context, input string) string {
	viper := config.Viper(ctx)

	replacer := strings.NewReplacer(
		viper.GetString(config.TokenLabel), "",
		viper.GetString(config.TokenSkip), "",
		viper.GetString(config.TokenForced), "",
	)

	return replacer.Replace(input)
}

// ValidateRequiredConfig checks viper and returns an error if a key isn't set
func ValidateRequiredConfig(ctx context.Context, opts ...string) error {
	viper := config.Viper(ctx)

	for _, opt := range opts {
		if viper.GetString(opt) == "" {
			return fmt.Errorf("%s is required - set as flag or env", opt)
		}
	}

	return nil
}

// ValidateEnumConfig validates that a config value is one of the allowed choices.
func ValidateEnumConfig(cmd *cobra.Command, key string, validChoices []string) error {
	viper := config.Viper(cmd.Context())

	// Validate the config value is one of the valid choices
	if outputStyle := viper.GetString(key); outputStyle != "" && !mapset.NewSet(validChoices...).Contains(outputStyle) {
		return fmt.Errorf("invalid %s: %q (expected one of %v)", key, viper.GetString(key), validChoices)
	}

	return nil
}
