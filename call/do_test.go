package call

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/output"
	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

// TestDo tests the Do function which orchestrates concurrent repository operations
func TestDo(t *testing.T) {
	tests := []struct {
		name          string
		repos         []string
		callFunc      Func
		channelBuffer int
		wantOutput    map[string]string // repo -> expected output
		wantError     bool
	}{
		{
			name:          "basic two repos",
			repos:         []string{"repo1", "repo2"},
			callFunc:      Wrap(fakeCallFunc(t, false, "test output for %s")),
			channelBuffer: 10,
			wantOutput: map[string]string{
				"repo1": "test output for repo1",
				"repo2": "test output for repo2",
			},
		},
		{
			name:          "single repo with buffering",
			repos:         []string{"repo1"},
			callFunc:      Wrap(fakeCallFunc(t, false, "test output for %s")),
			channelBuffer: 1,
			wantOutput: map[string]string{
				"repo1": "test output for repo1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			viper.Set(config.ChannelBuffer, tt.channelBuffer)

			// Create repo directories so Do won't try to clone
			testhelper.SetupDirs(t, ctx, tt.repos)

			var buf bytes.Buffer
			Do(fakeCmd(t, ctx, &buf), tt.repos, tt.callFunc)

			output := buf.String()

			testhelper.AssertContains(t, output, tt.wantOutput)

			for repo, got := range tt.wantOutput {
				if !strings.Contains(output, got) {
					t.Errorf("Expected output for %s to contain '%s'", repo, got)
				}
			}
		})
	}
}

// TestDoConcurrency tests the concurrency configuration of Do
func TestDoConcurrency(t *testing.T) {
	tests := []struct {
		name             string
		maxConcurrency   int
		expectedBehavior string
		fallbackToCPU    bool
	}{
		{
			name:             "CPU-based default concurrency",
			maxConcurrency:   0,
			expectedBehavior: "should fallback to CPU count",
			fallbackToCPU:    true,
		},
		{
			name:             "configured concurrency",
			maxConcurrency:   5,
			expectedBehavior: "should use configured value",
			fallbackToCPU:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			viper.Set(config.MaxConcurrency, tt.maxConcurrency)

			maxConcurrency := viper.GetInt(config.MaxConcurrency)
			if maxConcurrency <= 0 {
				maxConcurrency = runtime.NumCPU()
			}

			if tt.fallbackToCPU {
				expectedCPUs := runtime.NumCPU()
				if maxConcurrency != expectedCPUs {
					t.Errorf("Expected default concurrency to be %d (CPU count), got %d", expectedCPUs, maxConcurrency)
				}
			} else {
				if maxConcurrency != tt.maxConcurrency {
					t.Errorf("Expected configured concurrency to be %d, got %d", tt.maxConcurrency, maxConcurrency)
				}
			}
		})
	}
}

// TestDoBatching tests the batching behavior of Do with various concurrency limits
func TestDoBatching(t *testing.T) {
	tests := []struct {
		name               string
		maxConcurrency     int
		repos              []string
		expectedMaxWorkers int64
		workDuration       time.Duration
	}{
		{
			name:               "low concurrency limit",
			maxConcurrency:     2,
			repos:              []string{"repo1", "repo2", "repo3", "repo4", "repo5"},
			expectedMaxWorkers: 2,
			workDuration:       50 * time.Millisecond,
		},
		{
			name:               "high concurrency",
			maxConcurrency:     10,
			repos:              []string{"repo1", "repo2", "repo3"},
			expectedMaxWorkers: 10,
			workDuration:       10 * time.Millisecond,
		},
		{
			name:               "zero concurrency fallback",
			maxConcurrency:     0,
			repos:              []string{"repo1"},
			expectedMaxWorkers: 0, // Will fallback to CPU count
			workDuration:       10 * time.Millisecond,
		},
		{
			name:               "sync mode (concurrency=1)",
			maxConcurrency:     1,
			repos:              []string{"repo1", "repo2"},
			expectedMaxWorkers: 1,
			workDuration:       10 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			viper.Set(config.MaxConcurrency, tt.maxConcurrency)
			viper.Set(config.ChannelBuffer, 10)
			viper.Set(config.SortRepos, false)

			// Create repo directories so Do won't try to clone
			testhelper.SetupDirs(t, ctx, tt.repos)

			var activeWorkers int64
			var maxConcurrentWorkers int64
			var mutex sync.Mutex
			var processedCount int64

			var buf bytes.Buffer
			Do(fakeCmd(t, ctx, &buf), tt.repos, fakeCallFuncConcurrent(t, &activeWorkers, &maxConcurrentWorkers, &processedCount, &mutex, tt.workDuration))

			output := buf.String()

			// Verify all repos were processed
			testhelper.AssertContains(t, output, tt.repos)

			// Verify processed count
			if atomic.LoadInt64(&processedCount) != int64(len(tt.repos)) {
				t.Errorf("Expected %d repos to be processed, got %d", len(tt.repos), processedCount)
			}

			// Verify concurrency was limited appropriately
			if tt.expectedMaxWorkers > 0 && maxConcurrentWorkers > tt.expectedMaxWorkers {
				t.Errorf("Expected max concurrent workers to be %d, got %d", tt.expectedMaxWorkers, maxConcurrentWorkers)
			}

			if maxConcurrentWorkers == 0 && len(tt.repos) > 0 {
				t.Error("Expected at least one worker to be active")
			}
		})
	}
}

// TestProcessArguments tests the processArguments function which expands and sorts repos
func TestProcessArguments(t *testing.T) {
	tests := []struct {
		name        string
		sortRepos   bool
		args        []string
		want        []string
		wantBackoff time.Duration
		gitProvider string
	}{
		{
			name:        "basic processing without sorting",
			sortRepos:   false,
			args:        []string{"zebra", "alpha", "beta"},
			want:        []string{"zebra", "alpha", "beta"},
			wantBackoff: 1 * time.Second,
		},
		{
			name:        "sorting enabled",
			sortRepos:   true,
			args:        []string{"zebra", "alpha", "beta"},
			want:        []string{"alpha", "beta", "zebra"},
			wantBackoff: 1 * time.Second,
		},
		{
			name:        "github small backoff for few repos",
			sortRepos:   true,
			args:        []string{"repo1", "repo2"},
			want:        []string{"repo1", "repo2"},
			gitProvider: "github",
			wantBackoff: 2 * time.Second,
		},
		{
			name:        "github large backoff for many repos",
			sortRepos:   true,
			args:        []string{"repo01", "repo02", "repo03", "repo04", "repo05", "repo06", "repo07", "repo08", "repo09", "repo10", "repo11"},
			want:        []string{"repo01", "repo02", "repo03", "repo04", "repo05", "repo06", "repo07", "repo08", "repo09", "repo10", "repo11"},
			gitProvider: "github",
			wantBackoff: 8 * time.Second,
		},
		{
			name:        "default backoff for many repos (non-github)",
			sortRepos:   true,
			args:        []string{"repo01", "repo02", "repo03", "repo04", "repo05", "repo06", "repo07", "repo08", "repo09", "repo10", "repo11"},
			want:        []string{"repo01", "repo02", "repo03", "repo04", "repo05", "repo06", "repo07", "repo08", "repo09", "repo10", "repo11"},
			gitProvider: "bitbucket",
			wantBackoff: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			viper.Set(config.SortRepos, tt.sortRepos)

			// Set up provider-specific backoff config validation
			viper.Set(config.GitProvider, tt.gitProvider)
			viper.Set(config.GithubHourlyWriteLimit, 10)
			viper.Set(config.WriteBackoff, "1s")
			viper.Set(config.GithubBackoffSmall, "2s")
			viper.Set(config.GithubBackoffLarge, "8s")

			result := processArguments(ctx, tt.args)

			if tt.sortRepos {
				testhelper.AssertOutput(t, result, tt.want, nil, false)
			} else if len(result) != len(tt.want) {
				t.Errorf("Expected %d repos, got %d", len(tt.want), len(result))
			}

			if backoff := viper.GetDuration(config.WriteBackoff); backoff != tt.wantBackoff {
				t.Errorf("Expected backoff %v, got %v", tt.wantBackoff.String(), backoff.String())
			}
		})
	}
}

// TestDoVariousModes tests various execution modes of Do
func TestDoVariousModes(t *testing.T) {
	tests := []struct {
		name             string
		maxConcurrency   int
		repos            []string
		workDuration     time.Duration
		expectMaxWorkers int64
		checkTiming      bool
		expectedMaxTime  time.Duration
	}{
		{
			name:           "with nil writer (discard)",
			maxConcurrency: 5,
			repos:          []string{"repo1"},
		},
		{
			name:            "async with slow wrapper",
			maxConcurrency:  10,
			repos:           []string{"repo1", "repo2"},
			workDuration:    10 * time.Millisecond,
			checkTiming:     true,
			expectedMaxTime: 50 * time.Millisecond,
		},
		{
			name:             "sync flag behavior (concurrency=1)",
			maxConcurrency:   1,
			repos:            []string{"repo3", "repo1", "repo2"},
			workDuration:     50 * time.Millisecond,
			expectMaxWorkers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			viper.Set(config.MaxConcurrency, tt.maxConcurrency)
			viper.Set(config.ChannelBuffer, 10)
			viper.Set(config.SortRepos, false)

			// Create repo directories so Do won't try to clone
			testhelper.SetupDirs(t, ctx, tt.repos)

			var activeWorkers int64
			var maxConcurrentWorkers int64
			var mutex sync.Mutex
			var processedCount int64

			var buf bytes.Buffer
			start := time.Now()
			Do(fakeCmd(t, ctx, &buf), tt.repos, fakeCallFuncConcurrent(t, &activeWorkers, &maxConcurrentWorkers, &processedCount, &mutex, tt.workDuration))
			duration := time.Since(start)

			output := buf.String()

			// Verify all repos were processed
			testhelper.AssertContains(t, output, tt.repos)

			// Check timing if requested
			if tt.checkTiming && duration > tt.expectedMaxTime {
				t.Logf("Execution took %v, may not be truly async (expected < %v)", duration, tt.expectedMaxTime)
			}

			// Check max workers if specified
			if tt.expectMaxWorkers > 0 {
				actualMax := atomic.LoadInt64(&maxConcurrentWorkers)
				if actualMax != tt.expectMaxWorkers {
					t.Errorf("Expected max concurrent workers to be %d, got %d", tt.expectMaxWorkers, actualMax)
				}
			}
		})
	}
}

// TestDoWithContextCancellation tests that Do handles context cancellation properly
func TestDoWithContextCancellation(t *testing.T) {
	ctx := loadFixture(t)
	testhelper.SetupDirs(t, ctx, []string{"repo1", "repo2", "repo3"})

	viper := config.Viper(ctx)
	viper.Set(config.MaxConcurrency, 1) // Only 1 at a time
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	// Create a cancellable context
	cancelCtx, cancel := context.WithCancel(ctx)

	// Func that takes time and allows cancellation
	slowFunc := func(ctx context.Context, ch output.Channel) error {
		select {
		case <-time.After(100 * time.Millisecond):
			ch.WriteString("completed " + ch.Name())
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Cancel context immediately after starting
	cancel()

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, cancelCtx, &buf)
	cmd.SetErr(&errBuf)

	Do(cmd, []string{"repo1", "repo2", "repo3"}, slowFunc)

	errOutput := errBuf.String()

	// Should see context canceled error
	testhelper.AssertContains(t, errOutput, []string{"context canceled"})
}

// TestRunCallFuncCloning tests the repository cloning path in runCallFunc
func TestRunCallFuncCloning(t *testing.T) {
	ctx := loadFixture(t)

	viper := config.Viper(ctx)
	viper.Set(config.MaxConcurrency, 1)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	// Use a repo that doesn't exist - will attempt to clone
	missingRepo := "nonexistent-test-repo"

	testFunc := func(_ context.Context, ch output.Channel) error {
		ch.WriteString("executed for " + ch.Name())
		return nil
	}

	var buf, errBuf bytes.Buffer
	cmd := fakeCmd(t, ctx, &buf)
	cmd.SetErr(&errBuf)

	Do(cmd, []string{missingRepo}, testFunc)

	output := buf.String()
	errOutput := errBuf.String()

	// Should see git clone output and error from failed clone
	testhelper.AssertContains(t, output, []string{"Cloning into"})
	testhelper.AssertContains(t, errOutput, []string{"ERROR:"})
}
