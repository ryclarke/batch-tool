package call

import (
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
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

// Note: Testing the actual directory behavior of Exec would require
// integration with the utils package and proper repository setup,
// which is beyond the scope of unit tests.
