package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/cobra"
)

func TestCheckMutuallyExclusiveFlags(t *testing.T) {
	tests := []struct {
		name        string
		flags       map[string]bool // flag name -> value to set
		checkFlags  []string        // flags to check for mutual exclusivity
		expectError bool
	}{
		{
			name:        "no flags set",
			flags:       map[string]bool{},
			checkFlags:  []string{"flag-a", "flag-b"},
			expectError: false,
		},
		{
			name:        "only one flag set",
			flags:       map[string]bool{"flag-a": true},
			checkFlags:  []string{"flag-a", "flag-b"},
			expectError: false,
		},
		{
			name:        "both flags set - should error",
			flags:       map[string]bool{"flag-a": true, "flag-b": true},
			checkFlags:  []string{"flag-a", "flag-b"},
			expectError: true,
		},
		{
			name:        "three flags, two set - should error",
			flags:       map[string]bool{"flag-a": true, "flag-b": true},
			checkFlags:  []string{"flag-a", "flag-b", "flag-c"},
			expectError: true,
		},
		{
			name:        "three flags, one set - should pass",
			flags:       map[string]bool{"flag-a": true},
			checkFlags:  []string{"flag-a", "flag-b", "flag-c"},
			expectError: false,
		},
		{
			name:        "three flags, none set - should pass",
			flags:       map[string]bool{},
			checkFlags:  []string{"flag-a", "flag-b", "flag-c"},
			expectError: false,
		},
		{
			name:        "flag value false but explicitly set - counts as set",
			flags:       map[string]bool{"flag-a": false, "flag-b": true},
			checkFlags:  []string{"flag-a", "flag-b"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}

			// Register all flags that will be checked
			for _, flagName := range tt.checkFlags {
				cmd.Flags().Bool(flagName, false, "")
			}

			// Set the specified flags
			for flagName, value := range tt.flags {
				if err := cmd.Flags().Set(flagName, fmt.Sprintf("%t", value)); err != nil {
					t.Fatalf("Failed to set flag %s: %v", flagName, err)
				}
			}

			// Run the check
			err := CheckMutuallyExclusiveFlags(cmd, tt.checkFlags...)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}

			// Verify error message format when error is expected
			if tt.expectError && err != nil {
				errMsg := err.Error()
				if errMsg == "" {
					t.Error("Expected non-empty error message")
				}
				// Should contain "mutually exclusive"
				if !strings.Contains(errMsg, "mutually exclusive") {
					t.Errorf("Error message should mention mutual exclusivity, got: %q", errMsg)
				}
			}
		})
	}
}

func TestBuildBoolFlags(t *testing.T) {
	tests := []struct {
		name        string
		yesName     string
		yesShort    string
		noName      string
		noShort     string
		description string
	}{
		{
			name:        "both with short flags",
			yesName:     "enable",
			yesShort:    "e",
			noName:      "no-enable",
			noShort:     "n",
			description: "enable the feature",
		},
		{
			name:        "only yes with short flag",
			yesName:     "verbose",
			yesShort:    "v",
			noName:      "no-verbose",
			noShort:     "",
			description: "verbose output",
		},
		{
			name:        "neither with short flags",
			yesName:     "sort",
			yesShort:    "",
			noName:      "no-sort",
			noShort:     "",
			description: "sort the output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}

			BuildBoolFlags(cmd, tt.yesName, tt.yesShort, tt.noName, tt.noShort, tt.description)

			// Verify yes flag exists with correct defaults
			yesFlag := cmd.PersistentFlags().Lookup(tt.yesName)
			if yesFlag == nil {
				t.Errorf("Flag %q should exist", tt.yesName)
				return
			}
			if yesFlag.DefValue != "true" {
				t.Errorf("Flag %q default should be true, got %q", tt.yesName, yesFlag.DefValue)
			}
			if yesFlag.Usage != tt.description {
				t.Errorf("Flag %q usage should be %q, got %q", tt.yesName, tt.description, yesFlag.Usage)
			}

			// Verify no flag exists and is hidden
			noFlag := cmd.PersistentFlags().Lookup(tt.noName)
			if noFlag == nil {
				t.Errorf("Flag %q should exist", tt.noName)
				return
			}
			if noFlag.DefValue != "false" {
				t.Errorf("Flag %q default should be false, got %q", tt.noName, noFlag.DefValue)
			}
			if !noFlag.Hidden {
				t.Errorf("Flag %q should be hidden", tt.noName)
			}

			// Verify short flags if specified
			if tt.yesShort != "" {
				shortFlag := cmd.PersistentFlags().ShorthandLookup(tt.yesShort)
				if shortFlag == nil {
					t.Errorf("Short flag %q should exist for %q", tt.yesShort, tt.yesName)
				} else if shortFlag.Name != tt.yesName {
					t.Errorf("Short flag %q should map to %q, got %q", tt.yesShort, tt.yesName, shortFlag.Name)
				}
			}

			if tt.noShort != "" {
				shortFlag := cmd.PersistentFlags().ShorthandLookup(tt.noShort)
				if shortFlag == nil {
					t.Errorf("Short flag %q should exist for %q", tt.noShort, tt.noName)
				} else if shortFlag.Name != tt.noName {
					t.Errorf("Short flag %q should map to %q, got %q", tt.noShort, tt.noName, shortFlag.Name)
				}
			}
		})
	}
}

func TestBindBoolFlags(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		yesName        string
		noName         string
		setYesFlag     bool
		setNoFlag      bool
		noFlagValue    bool
		expectError    bool
		expectedConfig bool
	}{
		{
			name:           "no flags set - uses default",
			key:            "feature",
			yesName:        "enable",
			noName:         "no-enable",
			setYesFlag:     false,
			setNoFlag:      false,
			expectError:    false,
			expectedConfig: true, // yes flag default is true
		},
		{
			name:           "yes flag explicitly set",
			key:            "feature",
			yesName:        "enable",
			noName:         "no-enable",
			setYesFlag:     true,
			setNoFlag:      false,
			expectError:    false,
			expectedConfig: true,
		},
		{
			name:           "no flag set to true - inverts to false",
			key:            "feature",
			yesName:        "enable",
			noName:         "no-enable",
			setYesFlag:     false,
			setNoFlag:      true,
			noFlagValue:    true,
			expectError:    false,
			expectedConfig: false,
		},
		{
			name:           "no flag set to false - inverts to true",
			key:            "feature",
			yesName:        "enable",
			noName:         "no-enable",
			setYesFlag:     false,
			setNoFlag:      true,
			noFlagValue:    false,
			expectError:    false,
			expectedConfig: true,
		},
		{
			name:        "both flags set - error",
			key:         "feature",
			yesName:     "enable",
			noName:      "no-enable",
			setYesFlag:  true,
			setNoFlag:   true,
			noFlagValue: true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			// Create command with flags
			cmd := &cobra.Command{Use: "test"}
			cmd.SetContext(ctx)
			cmd.Flags().Bool(tt.yesName, true, "")
			cmd.Flags().Bool(tt.noName, false, "")

			// Set flags as needed
			if tt.setYesFlag {
				if err := cmd.Flags().Set(tt.yesName, "true"); err != nil {
					t.Fatalf("Failed to set yes flag: %v", err)
				}
			}
			if tt.setNoFlag {
				if err := cmd.Flags().Set(tt.noName, fmt.Sprintf("%t", tt.noFlagValue)); err != nil {
					t.Fatalf("Failed to set no flag: %v", err)
				}
			}

			// Run BindBoolFlags
			err := BindBoolFlags(cmd, tt.key, tt.yesName, tt.noName)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify config value
			got := viper.GetBool(tt.key)
			if got != tt.expectedConfig {
				t.Errorf("Config %q = %v, want %v", tt.key, got, tt.expectedConfig)
			}
		})
	}
}
