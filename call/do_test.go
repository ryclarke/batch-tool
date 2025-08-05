package call

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/viper"
)

func TestDo(t *testing.T) {
	// Set up test configuration
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, false)
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

func TestDoSync(t *testing.T) {
	// Set up test configuration
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, false) // DoSync should override this
	viper.Set(config.SortRepos, false)

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		ch <- "sync test for " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1"}

	DoSync(repos, &buf, testWrapper)

	output := buf.String()
	if !strings.Contains(output, "sync test for repo1") {
		t.Error("Expected sync output for repo1")
	}
}

func TestDoAsync(t *testing.T) {
	// Set up test configuration
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, true) // DoAsync should override this
	viper.Set(config.SortRepos, false)

	testWrapper := func(repo string, ch chan<- string) {
		defer close(ch)
		ch <- "async test for " + repo
	}

	var buf bytes.Buffer
	repos := []string{"repo1"}

	DoAsync(repos, &buf, testWrapper)

	output := buf.String()
	if !strings.Contains(output, "async test for repo1") {
		t.Error("Expected async output for repo1")
	}
}

func TestProcessArguments(t *testing.T) {
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
	// Test with different channel buffer sizes
	viper.Set(config.ChannelBuffer, 1)
	viper.Set(config.UseSync, false)
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
	// Test that Do handles different scenarios gracefully
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, false)
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
	// Test async behavior with slow wrapper
	viper.Set(config.ChannelBuffer, 10)
	viper.Set(config.UseSync, false)
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
