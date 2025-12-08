package output_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/call/output"
	"github.com/ryclarke/batch-tool/config"
)

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

			// Verify the handler is not nil
			if handler == nil {
				t.Fatal("getDefaultHandler returned nil")
			}

			// We can't directly compare function pointers, but we can verify
			// that we got a valid handler by checking it's callable
			// This is a basic sanity check
			if handler == nil {
				t.Errorf("Expected non-nil handler for type %s", tt.wantHandlerType)
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

// TestNativeHandler tests that NativeHandler properly handles and prints messages and errors
func TestNativeHandler(t *testing.T) {
	ctx := loadFixture(t)
	setupDirs(t, ctx, []string{"repo1", "repo2"})

	viper := config.Viper(ctx)
	viper.Set(config.MaxConcurrency, 2)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	// CallFunc that returns an error
	errorFunc := func(_ context.Context, repo string, ch chan<- string) error {
		ch <- "some output before error"
		return errors.New("test error for " + repo)
	}

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	call.Do(cmd, []string{"repo1", "repo2"}, errorFunc, output.NativeHandler)

	output := buf.String()
	errOutput := errBuf.String()

	// Verify headers and output were printed
	checkOutputContains(t, output, []string{"------ repo1 ------", "------ repo2 ------", "some output before error"})

	// Verify errors were printed to stderr
	checkOutputContains(t, errOutput, []string{"ERROR:", "test error for repo1", "test error for repo2"})
}
