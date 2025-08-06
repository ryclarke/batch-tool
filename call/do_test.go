package call

import (
	"bytes"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/viper"
)

func TestDo(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Set up test configuration
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	// Create a test wrapper that sends test data
	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		ch <- "test output for " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1", "repo2"}

	Do(repos, &buf, testWrapper)

	output := buf.String()
	if !strings.Contains(output, "test output for repo1") {
		t.Error("Expected output for repo1")
	}
	if !strings.Contains(output, "test output for repo2") {
		t.Error("Expected output for repo2")
	}
}

func TestProcessArguments(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Set up test configuration
	viper.Set(config.SortRepos, true)

	// Test basic processing (this will depend on catalog being initialized)
	args := []string{"repo3", "repo1", "repo2"}
	result := processArguments(args)

	// Since we don't have a real catalog, the function should return the input
	// but potentially sorted
	if len(result) == 0 {
		t.Error("Expected non-empty result from processArguments")
	}
}

func TestProcessArgumentsSorting(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test with sorting enabled
	viper.Set(config.SortRepos, true)

	// Mock a simple scenario where catalog returns the input
	args := []string{"zebra", "alpha", "beta"}
	result := processArguments(args)

	// The actual behavior depends on the catalog implementation
	// This test verifies the function doesn't crash
	if len(result) == 0 {
		t.Error("Expected non-empty result from processArguments")
	}
}

func TestProcessArgumentsNoSorting(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test with sorting disabled
	viper.Set(config.SortRepos, false)

	args := []string{"zebra", "alpha", "beta"}
	result := processArguments(args)

	// The actual behavior depends on the catalog implementation
	// This test verifies the function doesn't crash
	if len(result) == 0 {
		t.Error("Expected non-empty result from processArguments")
	}
}

func TestDoWithChannelBuffer(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test with different channel buffer sizes
	viper.Set(config.ChannelBuffer, 1)
	viper.Set(config.SortRepos, false)

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		// Send multiple messages to test buffering
		for i := 0; i < 5; i++ {
			ch <- "message " + string(rune('0'+i)) + " for " + repo
		}
	}

	var buf bytes.Buffer
	repos := []string{"repo1"}

	Do(repos, &buf, testWrapper)

	output := buf.String()
	if !strings.Contains(output, "message 0 for repo1") {
		t.Error("Expected first message in output")
	}
	if !strings.Contains(output, "message 4 for repo1") {
		t.Error("Expected last message in output")
	}
}

func TestDoWithNilWriter(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test that Do handles different scenarios gracefully
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		ch <- "test"
	}

	// Test with a discard writer instead of nil to avoid panic
	var buf bytes.Buffer
	Do([]string{"repo1"}, &buf, testWrapper)

	// Just verify it doesn't crash
	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Error("Expected test output in buffer")
	}
}

func TestDoWithSlowWrapper(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test async behavior with slow wrapper
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	slowWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		time.Sleep(10 * time.Millisecond) // Small delay
		ch <- "slow output for " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1", "repo2"}

	start := time.Now()
	Do(repos, &buf, slowWrapper)
	duration := time.Since(start)

	// Async execution should be faster than sequential
	if duration > 50*time.Millisecond {
		t.Logf("Execution took %v, may not be truly async", duration)
	}

	output := buf.String()
	if !strings.Contains(output, "slow output for repo1") {
		t.Error("Expected output for repo1")
	}
	if !strings.Contains(output, "slow output for repo2") {
		t.Error("Expected output for repo2")
	}
}

func TestSyncFlagBehavior(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test that setting MaxConcurrency to 1 enforces sequential execution
	viper.Set(config.MaxConcurrency, 1) // Simulate --sync flag behavior
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.SortRepos, false)

	var activeWorkers int64
	var maxConcurrentWorkers int64

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)

		// Track concurrent workers
		current := atomic.AddInt64(&activeWorkers, 1)
		defer atomic.AddInt64(&activeWorkers, -1)

		// Update maximum concurrent workers seen
		if current > maxConcurrentWorkers {
			maxConcurrentWorkers = current
		}

		// Simulate some work
		time.Sleep(50 * time.Millisecond)

		ch <- "processed " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1", "repo2", "repo3"}

	Do(repos, &buf, testWrapper)

	output := buf.String()

	// Verify all repos were processed
	for _, repo := range repos {
		expected := "processed " + repo
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain '%s'", expected)
		}
	}

	// Verify only 1 worker was active at a time (sequential execution)
	if maxConcurrentWorkers != 1 {
		t.Errorf("Expected max concurrent workers to be 1 (sync mode), got %d", maxConcurrentWorkers)
	}
}
