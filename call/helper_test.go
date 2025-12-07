package call

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	"github.com/spf13/cobra"
)

func loadFixture(t *testing.T) context.Context {
	t.Helper()
	return config.LoadFixture(t, "../config")
}

// checkError validates test error against expected results
func checkError(t *testing.T, err error, wantErr bool) bool {
	t.Helper()

	if wantErr != (err != nil) {
		t.Errorf("Expected error = %v, got: %v", wantErr, err)
		return false
	}

	return true
}

// checkOutput validates test output and error against expected values
func checkOutput(t *testing.T, got, want []string, err error, wantErr bool) {
	t.Helper()

	if !checkError(t, err, wantErr) {
		return
	}

	// Validate that the output matches what's expected
	if len(got) != len(want) {
		t.Errorf("Expected %d output messages, got %d", len(want), len(got))
		return
	}

	for i, msg := range want {
		if got[i] != msg {
			t.Errorf("Expected output[%d] = %s, got %s", i, msg, got[i])
		}
	}
}

// checkOutputContains checks that all expected messages are present in the output (supports map or slice)
func checkOutputContains(t *testing.T, got string, want any) {
	t.Helper()

	switch wantIter := want.(type) {
	case map[string]string:
		for key, msg := range wantIter {
			if !strings.Contains(got, msg) {
				t.Errorf("Expected output for %s to contain %s", key, msg)
			}
		}
	case []string:
		for _, msg := range wantIter {
			if !strings.Contains(got, msg) {
				t.Errorf("Expected output to contain %s", msg)
			}
		}
	}
}

// setupDirs creates the repository directories so that Do won't try to clone them
func setupDirs(t *testing.T, ctx context.Context, repos []string) {
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

// fakeCmd creates a test cobra command with the given context and output writer
func fakeCmd(t *testing.T, ctx context.Context, out io.Writer) *cobra.Command {
	t.Helper()

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.SetOut(out)

	return cmd
}

// fakeCallFunc returns a CallFunc that sends the specified output messages to the channel.
func fakeCallFunc(t *testing.T, wantErr bool, output ...string) CallFunc {
	t.Helper()

	return func(_ context.Context, repo string, ch chan<- string) error {
		for _, msg := range output {
			if strings.Contains(msg, "%s") {
				msg = fmt.Sprintf(msg, repo)
			}

			ch <- msg
		}

		if wantErr {
			return errors.New("test error")
		}

		return nil
	}
}

func fakeCallFuncConcurrent(t *testing.T, activeWorkers, maxConcurrent, count *int64, mutex *sync.Mutex, workDuration time.Duration) CallFunc {
	t.Helper()

	return func(_ context.Context, repo string, ch chan<- string) error {
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

		ch <- repo
		return nil
	}
}
