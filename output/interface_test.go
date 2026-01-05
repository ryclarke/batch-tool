package output_test

import (
	"context"
	"io"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func loadFixture(t *testing.T) context.Context {
	t.Helper()
	return testhelper.LoadFixture(t, "../config")
}

// fakeCmd creates a minimal cobra.Command for testing with the given context and output buffer
func fakeCmd(t *testing.T, ctx context.Context, out io.Writer) *cobra.Command {
	return testhelper.FakeCmd(t, ctx, out)
}

// TestGetHandler tests the GetHandler function
func TestGetHandler(t *testing.T) {
	tests := []struct {
		name            string
		configValue     string
		wantHandlerType string
	}{
		{
			name:            "native handler selected",
			configValue:     "native",
			wantHandlerType: "native",
		},
		{
			name:            "bubbletea handler selected",
			configValue:     "bubbletea",
			wantHandlerType: "bubbletea",
		},
		{
			name:            "empty value defaults to native",
			configValue:     "",
			wantHandlerType: "native",
		},
		{
			name:            "invalid value defaults to native",
			configValue:     "invalid-handler",
			wantHandlerType: "native",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			// Set the handler type
			if tt.configValue != "" {
				viper.Set(config.OutputStyle, tt.configValue)
			}

			handler := output.GetHandler(ctx)

			// We can't directly compare function pointers, but we can verify
			// that we got a valid handler by checking it's callable
			// This is a basic sanity check
			if handler == nil {
				t.Errorf("Expected non-nil handler for type %s", tt.wantHandlerType)
			}
		})
	}
}

// TestGetLabels tests the GetLabels function
func TestGetLabels(t *testing.T) {
	tests := []struct {
		name        string
		configValue string
	}{
		{
			name:        "native label handler selected",
			configValue: "native",
		},
		{
			name:        "bubbletea label handler selected",
			configValue: "bubbletea",
		},
		{
			name:        "empty value defaults to native",
			configValue: "",
		},
		{
			name:        "invalid value defaults to native",
			configValue: "invalid-handler",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			// Set the handler type
			if tt.configValue != "" {
				viper.Set(config.OutputStyle, tt.configValue)
			}

			labelHandler := output.GetLabelHandler(ctx)

			// Verify the handler is callable
			if labelHandler == nil {
				t.Errorf("Expected non-nil label handler")
			}
		})
	}
}

// TestGetHandlerIntegration tests that the handler configuration integrates properly
func TestGetHandlerIntegration(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Test that we can get a handler with default config
	handler := output.GetHandler(ctx)
	if handler == nil {
		t.Fatal("getDefaultHandler returned nil with default config")
	}

	// Test that we can change the handler type
	viper.Set(config.OutputStyle, "bubbletea")
	handler = output.GetHandler(ctx)
	if handler == nil {
		t.Fatal("getDefaultHandler returned nil with bubbletea config")
	}
}
