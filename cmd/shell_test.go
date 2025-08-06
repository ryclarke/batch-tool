package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestAddShellCmd(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addShellCmd()

	if cmd == nil {
		t.Fatal("addShellCmd() returned nil")
	}

	if cmd.Use != "shell <repository> ..." {
		t.Errorf("Expected Use to be 'shell <repository> ...', got %s", cmd.Use)
	}

	// Test aliases
	aliases := cmd.Aliases
	if len(aliases) != 1 || aliases[0] != "sh" {
		t.Errorf("Expected aliases to contain 'sh', got %v", aliases)
	}

	if cmd.Short != "[!DANGEROUS!] Execute a shell command across repositories" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestShellCmdFlags(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addShellCmd()

	// Test exec flag
	execFlag := cmd.Flags().Lookup("exec")
	if execFlag == nil {
		t.Error("exec flag not found")
	}

	if execFlag.Shorthand != "c" {
		t.Errorf("Expected exec flag shorthand to be 'c', got %s", execFlag.Shorthand)
	}

	if execFlag.Usage != "shell command(s) to execute" {
		t.Errorf("Expected correct usage description, got %s", execFlag.Usage)
	}
}

func TestShellCmdArgs(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addShellCmd()

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

func TestShellCmdHelp(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addShellCmd()

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
	if !strings.Contains(output, "[!DANGEROUS!]") {
		t.Error("Help output should contain danger warning")
	}

	if !strings.Contains(output, "--exec") {
		t.Error("Help output should contain exec flag information")
	}
}

func TestShellCmdRunEErrorHandling(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addShellCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test with missing exec flag
	cmd.SetArgs([]string{"repo1"})
	err := cmd.RunE(cmd, []string{"repo1"})

	// Should return an error when exec flag is not set
	if err == nil {
		t.Error("Expected error when exec flag is not provided")
	}
}

func TestShellCmdWithExecFlag(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addShellCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Set the exec flag
	cmd.Flags().Set("exec", "echo test")

	// Test that we can get the exec value
	execValue, err := cmd.Flags().GetString("exec")
	if err != nil {
		t.Errorf("Failed to get exec flag value: %v", err)
	}

	if execValue != "echo test" {
		t.Errorf("Expected exec value to be 'echo test', got %s", execValue)
	}
}

// Note: We can't easily test the interactive confirmation part of the shell command
// without mocking stdin, which would require more complex test setup
