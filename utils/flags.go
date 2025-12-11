package utils

import (
	"fmt"
	"strings"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/cobra"
)

// BuildBoolFlags adds a pair of mutually exclusive boolean flags to the given command.
// The "yes" flag enables the feature (default true), and the "no" flag disables it.
func BuildBoolFlags(cmd *cobra.Command, yesName, yesShort, noName, noShort, description string) {
	if yesShort != "" {
		cmd.PersistentFlags().BoolP(yesName, yesShort, true, description)
	} else {
		cmd.PersistentFlags().Bool(yesName, true, description)
	}

	if noShort != "" {
		cmd.PersistentFlags().BoolP(noName, noShort, false, "")
	} else {
		cmd.PersistentFlags().Bool(noName, false, "")
	}

	cmd.PersistentFlags().MarkHidden(noName)
}

// BindBoolFlags binds a pair of mutually exclusive boolean flags to the current viper context.
func BindBoolFlags(cmd *cobra.Command, key, yesName, noName string) error {
	viper := config.Viper(cmd.Context())
	viper.BindPFlag(key, cmd.Flags().Lookup(yesName))

	// Only allow one of the flags in the pair to be set
	if err := CheckMutuallyExclusiveFlags(cmd, yesName, noName); err != nil {
		return err
	}

	// Override the value if the inverted flag is explicitly set
	if cmd.Flags().Changed(noName) {
		noValue, err := cmd.Flags().GetBool(noName)
		if err != nil {
			return err
		}

		viper.Set(key, !noValue)
	}

	return nil
}

// CheckMutuallyExclusiveFlags validates that at most one flag from each set of mutually exclusive flags is set.
// Each argument is a slice of flag names that are mutually exclusive with each other.
// Returns an error if more than one flag in any set is explicitly set.
func CheckMutuallyExclusiveFlags(cmd *cobra.Command, flags ...string) error {
	var setFlags []string

	for _, flagName := range flags {
		if cmd.Flags().Changed(flagName) {
			setFlags = append(setFlags, "--"+flagName)
		}
	}

	if len(setFlags) > 1 {
		return fmt.Errorf("mutually exclusive flags cannot be used together: %s", strings.Join(setFlags, ", "))
	}

	return nil
}
