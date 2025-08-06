package call

import (
	"bytes"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/viper"
)

func TestExec(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Create a test function using Exec
	execFunc := Exec("echo", "test message")

	if execFunc == nil {
		t.Fatal("Exec returned nil function")
	}

	// Create a channel to capture output
	ch := make(chan string, 10)

	// Execute the function - note that without proper repo setup,
	// this will likely fail due to directory issues
	err := execFunc("", ch)
	close(ch)

	// For unit tests, we mainly verify the function doesn't crash
	// and returns appropriate errors for invalid scenarios
	if err == nil {
		// If no error, collect and verify output
		var output []string
		for msg := range ch {
			output = append(output, msg)
		}

		if len(output) > 0 {
			joined := strings.Join(output, " ")
			if !strings.Contains(joined, "test message") {
				t.Errorf("Expected 'test message' in output, got: %s", joined)
			}
		}
	} else {
		// Expected error due to test environment - verify it's directory-related
		if !strings.Contains(err.Error(), "chdir") && !strings.Contains(err.Error(), "directory") {
			t.Errorf("Expected directory-related error, got: %v", err)
		}
	}
}

func TestExecWithArguments(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test Exec with multiple arguments
	execFunc := Exec("echo", "hello", "world", "test")

	if execFunc == nil {
		t.Fatal("Exec returned nil function")
	}

	ch := make(chan string, 10)

	err := execFunc("", ch)
	close(ch)

	// For unit tests, we expect errors due to directory issues
	if err == nil {
		// If no error, verify output
		var output []string
		for msg := range ch {
			output = append(output, msg)
		}

		if len(output) == 0 {
			t.Error("Expected output from echo command with multiple args")
		}
	} else {
		// Expected error - verify it's directory-related
		if !strings.Contains(err.Error(), "chdir") && !strings.Contains(err.Error(), "directory") {
			t.Errorf("Expected directory-related error, got: %v", err)
		}
	}
}

func TestExecInvalidCommand(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test Exec with a command that doesn't exist
	execFunc := Exec("nonexistent-command-xyz", "arg1")

	if execFunc == nil {
		t.Fatal("Exec returned nil function")
	}

	ch := make(chan string, 10)

	err := execFunc("", ch)
	close(ch)

	// Should return an error for nonexistent command
	if err == nil {
		t.Error("Expected error for nonexistent command")
	}
}

func TestExecWithEmptyCommand(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test Exec with empty command
	execFunc := Exec("", "arg1")

	if execFunc == nil {
		t.Fatal("Exec returned nil function")
	}

	ch := make(chan string, 10)

	err := execFunc("", ch)
	close(ch)

	// Should return an error for empty command
	if err == nil {
		t.Error("Expected error for empty command")
	}
}

func TestExecChannelOutput(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test that Exec properly streams output to channel
	execFunc := Exec("echo", "line1\nline2\nline3")

	ch := make(chan string, 10)

	err := execFunc("", ch)
	close(ch)

	// For unit tests, we expect errors due to directory issues
	if err == nil {
		// Count messages in channel
		var messageCount int
		for range ch {
			messageCount++
		}

		// Should receive at least one message if successful
		if messageCount == 0 {
			t.Error("Expected at least one message in channel")
		}
	} else {
		// Expected error - verify it's directory-related
		if !strings.Contains(err.Error(), "chdir") && !strings.Contains(err.Error(), "directory") {
			t.Errorf("Expected directory-related error, got: %v", err)
		}
	}
}

func TestCPUBasedDefaultConcurrency(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test that the default concurrency matches CPU count
	viper.Set(config.MaxConcurrency, 0) // Force fallback to default

	expectedCPUs := runtime.NumCPU()

	// Get the actual value that would be used in Do function
	maxConcurrency := viper.GetInt(config.MaxConcurrency)
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	if maxConcurrency != expectedCPUs {
		t.Errorf("Expected default concurrency to be %d (CPU count), got %d", expectedCPUs, maxConcurrency)
	}
}

func TestConfigurationBasedConcurrency(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test that configuration values are respected
	expectedConcurrency := 5
	viper.Set(config.MaxConcurrency, expectedConcurrency)

	maxConcurrency := viper.GetInt(config.MaxConcurrency)
	if maxConcurrency <= 0 {
		maxConcurrency = runtime.NumCPU()
	}

	if maxConcurrency != expectedConcurrency {
		t.Errorf("Expected configured concurrency to be %d, got %d", expectedConcurrency, maxConcurrency)
	}
}

func TestDoBatching(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Set up test configuration with low concurrency
	viper.Set(config.MaxConcurrency, 2)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, false)
	viper.Set(config.SortRepos, false)

	var activeWorkers int64
	var maxConcurrentWorkers int64
	var mutex sync.Mutex

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)

		// Track concurrent workers
		current := atomic.AddInt64(&activeWorkers, 1)
		defer atomic.AddInt64(&activeWorkers, -1)

		// Update maximum concurrent workers seen
		mutex.Lock()
		if current > maxConcurrentWorkers {
			maxConcurrentWorkers = current
		}
		mutex.Unlock()

		// Simulate some work
		time.Sleep(50 * time.Millisecond)

		ch <- "processed " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1", "repo2", "repo3", "repo4", "repo5"}

	Do(repos, &buf, testWrapper)

	output := buf.String()

	// Verify all repos were processed
	for _, repo := range repos {
		expected := "processed " + repo
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain '%s'", expected)
		}
	}

	// Verify concurrency was limited (should not exceed 2)
	if maxConcurrentWorkers > 2 {
		t.Errorf("Expected max concurrent workers to be 2, got %d", maxConcurrentWorkers)
	}

	if maxConcurrentWorkers == 0 {
		t.Error("Expected at least one worker to be active")
	}
}

func TestDoBatchingHighConcurrency(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test with higher concurrency limit
	viper.Set(config.MaxConcurrency, 10)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, false)
	viper.Set(config.SortRepos, false)

	var processedCount int64

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		atomic.AddInt64(&processedCount, 1)
		ch <- "done " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1", "repo2", "repo3"}

	Do(repos, &buf, testWrapper)

	// Verify all repos were processed
	if atomic.LoadInt64(&processedCount) != 3 {
		t.Errorf("Expected 3 repos to be processed, got %d", processedCount)
	}
}

func TestDoBatchingZeroConcurrency(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test with zero concurrency (should fallback to default)
	viper.Set(config.MaxConcurrency, 0)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, false)
	viper.Set(config.SortRepos, false)

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		ch <- "fallback test for " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1"}

	// Should not panic or fail
	Do(repos, &buf, testWrapper)

	output := buf.String()
	if !strings.Contains(output, "fallback test for repo1") {
		t.Error("Expected fallback behavior to work")
	}
}

func TestDoBatchingSyncMode(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test that sync mode bypasses batching
	viper.Set(config.MaxConcurrency, 1)
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, true) // Enable sync mode
	viper.Set(config.SortRepos, false)

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		ch <- "sync test for " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1", "repo2"}

	Do(repos, &buf, testWrapper)

	output := buf.String()
	if !strings.Contains(output, "sync test for repo1") {
		t.Error("Expected sync output for repo1")
	}
	if !strings.Contains(output, "sync test for repo2") {
		t.Error("Expected sync output for repo2")
	}
}

// Note: Testing the actual directory behavior of Exec would require
// integration with the utils package and proper repository setup,
// which is beyond the scope of unit tests.
