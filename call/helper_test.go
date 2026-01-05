package call

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/output"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func loadFixture(t *testing.T) context.Context {
	t.Helper()
	return testhelper.LoadFixture(t, "../config")
}

// fakeCmd creates a test cobra command with the given context and output writer
func fakeCmd(t *testing.T, ctx context.Context, out io.Writer) *cobra.Command {
	return testhelper.FakeCmd(t, ctx, out)
}

// fakeCallFunc returns a Func that sends the specified output messages to the channel.
func fakeCallFunc(t *testing.T, wantErr bool, wantOutput ...string) Func {
	t.Helper()

	return func(_ context.Context, ch output.Channel) error {
		for _, msg := range wantOutput {
			if strings.Contains(msg, "%s") {
				msg = fmt.Sprintf(msg, ch.Name())
			}

			ch.WriteString(msg)
		}

		if wantErr {
			return errors.New("test error")
		}

		return nil
	}
}

func fakeCallFuncConcurrent(t *testing.T, activeWorkers, maxConcurrent, count *int64, mutex *sync.Mutex, workDuration time.Duration) Func {
	t.Helper()

	return func(_ context.Context, ch output.Channel) error {
		// Track concurrent workers
		current := atomic.AddInt64(activeWorkers, 1)
		defer atomic.AddInt64(activeWorkers, -1)
		atomic.AddInt64(count, 1)

		// Update maximum concurrent workers seen
		mutex.Lock()
		if current > *maxConcurrent {
			*maxConcurrent = current
		}
		mutex.Unlock()

		// Simulate some work
		if workDuration > 0 {
			time.Sleep(workDuration)
		}

		ch.WriteString(ch.Name())
		return nil
	}
}
