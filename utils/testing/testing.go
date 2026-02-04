// Package testing provides utility functions for testing purposes across multiple packages.
package testing

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
)

// LoadFixture loads test configuration from the config package.
// The configPath parameter should be the relative path from the test file
// to the config directory (e.g., "../config", "../../config").
func LoadFixture(t *testing.T, configPath string) context.Context {
	t.Helper()

	viper := config.New()
	ctx := config.SetViper(context.Background(), viper)

	viper.SetConfigName("fixture")
	viper.AddConfigPath(configPath)

	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("Failed to load fixture config: %v", err)
	}

	// Override GitDirectory with a temporary directory for test isolation and reliable cleanup
	tempDir := t.TempDir()
	viper.Set(config.GitDirectory, tempDir)

	return ctx
}

// SetupDirs creates repository directories to prevent cloning during tests.
// Automatically registers cleanup to remove the directories after the test.
func SetupDirs(t *testing.T, ctx context.Context, repos []string) {
	t.Helper()

	for _, repo := range repos {
		repoPath := utils.RepoPath(ctx, repo)
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			t.Fatalf("Failed to create repo directory %s: %v", repoPath, err)
		}
	}

	// Clean up after test
	t.Cleanup(func() {
		gitDir := config.Viper(ctx).GetString(config.GitDirectory)
		os.RemoveAll(gitDir)
	})
}

// FakeCmd creates a minimal cobra.Command for testing with the given context and output writer.
func FakeCmd(t *testing.T, ctx context.Context, out io.Writer) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{
		Use: "test",
	}
	cmd.SetContext(ctx)
	cmd.SetOut(out)
	cmd.SetErr(out)

	return cmd
}

// MockChannel implements output.Channel for testing purposes.
// It provides a simple channel implementation that captures output
// and can be used to test call.Func implementations directly.
type MockChannel struct {
	name   string
	output []byte
	err    error
}

// NewMockChannel creates a new MockChannel with the given name.
func NewMockChannel(name string) *MockChannel {
	return &MockChannel{name: name}
}

// Name returns the channel name.
func (m *MockChannel) Name() string {
	return m.name
}

// Out returns nil (not used in mock).
func (m *MockChannel) Out() <-chan []byte {
	return nil
}

// Err returns nil (not used in mock).
func (m *MockChannel) Err() <-chan error {
	return nil
}

// WriteString writes a string to the mock output.
func (m *MockChannel) WriteString(s string) (int, error) {
	m.output = append(m.output, []byte(s)...)
	return len(s), nil
}

// Write writes bytes to the mock output.
func (m *MockChannel) Write(p []byte) (int, error) {
	m.output = append(m.output, p...)
	return len(p), nil
}

// WriteError stores an error in the mock.
func (m *MockChannel) WriteError(err error) {
	m.err = err
}

// Start is a no-op for the mock.
func (m *MockChannel) Start(_ int64) error {
	return nil
}

// Close is a no-op for the mock.
func (m *MockChannel) Close() error {
	return nil
}

// Output returns the captured output.
func (m *MockChannel) Output() []byte {
	return m.output
}

// Error returns any captured error.
func (m *MockChannel) Error() error {
	return m.err
}
