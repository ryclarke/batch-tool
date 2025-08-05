package git

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddCommitCmd(t *testing.T) {
	cmd := addCommitCmd()

	if cmd == nil {
		t.Fatal("addCommitCmd() returned nil")
	}

	if cmd.Use != "commit <repository> ..." {
		t.Errorf("Expected Use to be 'commit <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Commit code changes across repositories" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestCommitCmdFlags(t *testing.T) {
	cmd := addCommitCmd()

	// Test amend flag
	amendFlag := cmd.Flags().Lookup("amend")
	if amendFlag == nil {
		t.Error("amend flag not found")
	}

	if amendFlag.Shorthand != "a" {
		t.Errorf("Expected amend flag shorthand to be 'a', got %s", amendFlag.Shorthand)
	}

	// Test message flag
	messageFlag := cmd.Flags().Lookup("message")
	if messageFlag == nil {
		t.Error("message flag not found")
	}

	if messageFlag.Shorthand != "m" {
		t.Errorf("Expected message flag shorthand to be 'm', got %s", messageFlag.Shorthand)
	}
}

func TestCommitCmdArgs(t *testing.T) {
	cmd := addCommitCmd()

	// Test that command requires minimum arguments
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("Expected error when no arguments provided")
	}

	// Test that command accepts arguments
	err = cmd.Args(cmd, []string{"repo1"})
	if err != nil {
		t.Errorf("Expected no error with valid arguments, got %v", err)
	}
}

func TestCommitCmdHelp(t *testing.T) {
	cmd := addCommitCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Set help flag and execute
	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("Help execution failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Commit code changes") {
		t.Error("Help output should contain command description")
	}

	if !strings.Contains(output, "--amend") {
		t.Error("Help output should contain amend flag information")
	}

	if !strings.Contains(output, "--message") {
		t.Error("Help output should contain message flag information")
	}
}

func TestCommitCmdPreRunE(t *testing.T) {
	cmd := addCommitCmd()

	// Test PreRunE function exists
	if cmd.PreRunE == nil {
		t.Error("Expected PreRunE function to be set")
		return
	}

	// Test with amend flag set (should not require message)
	cmd.Flags().Set("amend", "true")
	err := cmd.PreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("Expected no error when amend flag is set, got %v", err)
	}
}

// Note: Testing the actual git operations would require setting up a test git repository
// which is beyond the scope of unit tests. Integration tests would be more appropriate
// for testing the actual git command execution.
