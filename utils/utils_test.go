package utils_test

import (
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	testhelper "github.com/ryclarke/batch-tool/utils/test"
	"github.com/spf13/cobra"
)

func TestCleanFilter(t *testing.T) {
	ctx := loadFixture(t)

	tests := []struct {
		name       string
		filter     string
		wantResult string
	}{
		{
			name:       "remove label token",
			filter:     "~frontend",
			wantResult: "frontend",
		},
		{
			name:       "remove skip token",
			filter:     "!backend",
			wantResult: "backend",
		},
		{
			name:       "remove forced token",
			filter:     "+deprecated",
			wantResult: "deprecated",
		},
		{
			name:       "remove multiple tokens",
			filter:     "+~frontend",
			wantResult: "frontend",
		},
		{
			name:       "remove skip and label tokens",
			filter:     "!~backend",
			wantResult: "backend",
		},
		{
			name:       "plain name unchanged",
			filter:     "web-app",
			wantResult: "web-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.CleanFilter(ctx, tt.filter)
			testhelper.AssertEqual(t, result, tt.wantResult)
		})
	}
}

func TestValidateRequiredConfig(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	tests := []struct {
		name      string
		setup     func()
		keys      []string
		wantError bool
	}{
		{
			name:      "missing required config",
			keys:      []string{"nonexistent.key"},
			wantError: true,
		},
		{
			name: "existing config",
			setup: func() {
				viper.Set("test.key", "test-value")
			},
			keys:      []string{"test.key"},
			wantError: false,
		},
		{
			name: "multiple keys some missing",
			setup: func() {
				viper.Set("test.key", "test-value")
			},
			keys:      []string{"test.key", "missing.key"},
			wantError: true,
		},
		{
			name: "multiple keys all present",
			setup: func() {
				viper.Set("test.key", "test-value")
				viper.Set("test.key2", "test-value2")
			},
			keys:      []string{"test.key", "test.key2"},
			wantError: false,
		},
		{
			name: "empty string value treated as missing",
			setup: func() {
				viper.Set("empty.key", "")
			},
			keys:      []string{"empty.key"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			err := utils.ValidateRequiredConfig(ctx, tt.keys...)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateRequiredConfig() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateEnumConfig(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		configValue  string
		validChoices []string
		expectError  bool
	}{
		{
			name:         "valid choice",
			key:          "output-style",
			configValue:  "json",
			validChoices: []string{"json", "yaml", "text"},
			expectError:  false,
		},
		{
			name:         "empty value - should pass",
			key:          "output-style",
			configValue:  "",
			validChoices: []string{"json", "yaml", "text"},
			expectError:  false,
		},
		{
			name:         "invalid choice",
			key:          "output-style",
			configValue:  "xml",
			validChoices: []string{"json", "yaml", "text"},
			expectError:  true,
		},
		{
			name:         "single valid choice",
			key:          "format",
			configValue:  "table",
			validChoices: []string{"table"},
			expectError:  false,
		},
		{
			name:         "invalid with single choice",
			key:          "format",
			configValue:  "list",
			validChoices: []string{"table"},
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			// Set config value
			if tt.configValue != "" {
				viper.Set(tt.key, tt.configValue)
			}

			// Create command
			cmd := &cobra.Command{Use: "test"}
			cmd.SetContext(ctx)

			// Run check
			err := utils.ValidateEnumConfig(cmd, tt.key, tt.validChoices)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), "invalid") {
					t.Errorf("Error should contain 'invalid', got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
