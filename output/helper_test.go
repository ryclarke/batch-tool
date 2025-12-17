package output

import (
	"context"
	"testing"

	testhelper "github.com/ryclarke/batch-tool/utils/test"
	"github.com/spf13/cobra"
)

func loadFixture(t *testing.T) context.Context {
	t.Helper()
	return testhelper.LoadFixture(t, "../config")
}

// testCancelFunc is a no-op cancel function for tests
func testCancelFunc() { /* no-op */ }

// makeTestCommand creates a command with a properly initialized context for testing
func makeTestCommand(t *testing.T) *cobra.Command {
	t.Helper()

	ctx := loadFixture(t)
	return testhelper.FakeCmd(t, ctx, nil)
}

// testChannel implements the Channel interface for testing
type testChannel struct {
	name   string
	output chan string
	err    chan error
}

func (tc *testChannel) Name() string       { return tc.name }
func (tc *testChannel) Out() <-chan string { return tc.output }
func (tc *testChannel) Err() <-chan error  { return tc.err }
func (tc *testChannel) WriteString(s string) (n int, _ error) {
	tc.output <- s
	return len(s), nil
}
func (tc *testChannel) WriteError(err error) {
	tc.err <- err
}
func (tc *testChannel) Start(weight int64) error { return nil }
func (tc *testChannel) Close() error {
	close(tc.output)
	close(tc.err)
	return nil
}

// makeTestChannels creates test channels for the given repo names.
// If closed is true, the channels are immediately closed to avoid blocking in tests.
func makeTestChannels(names []string, closed bool) []Channel {
	channels := make([]Channel, len(names))
	for i, name := range names {
		tc := &testChannel{
			name:   name,
			output: make(chan string),
			err:    make(chan error),
		}
		if closed {
			close(tc.output)
			close(tc.err)
		}
		channels[i] = tc
	}

	return channels
}
