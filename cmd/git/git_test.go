package git

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddStatusCmd(t *testing.T) {
	cmd := addStatusCmd()

	if cmd == nil {
		t.Fatal("addStatusCmd() returned nil")
	}

	if cmd.Use != "status <repository> ..." {
		t.Errorf("Expected Use to be 'status <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Git status of each repository" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestStatusCmdArgs(t *testing.T) {
	cmd := addStatusCmd()

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

func TestStatusCmdHelp(t *testing.T) {
	cmd := addStatusCmd()

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
	if !strings.Contains(output, "Git status of each repository") {
		t.Error("Help output should contain command description")
	}
}

func TestAddDiffCmd(t *testing.T) {
	cmd := addDiffCmd()

	if cmd == nil {
		t.Fatal("addDiffCmd() returned nil")
	}

	if cmd.Use != "diff <repository> ..." {
		t.Errorf("Expected Use to be 'diff <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Git diff of each repository" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestDiffCmdArgs(t *testing.T) {
	cmd := addDiffCmd()

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

func TestDiffCmdHelp(t *testing.T) {
	cmd := addDiffCmd()

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
	if !strings.Contains(output, "Git diff of each repository") {
		t.Error("Help output should contain command description")
	}
}

func TestAddUpdateCmd(t *testing.T) {
	cmd := addUpdateCmd()

	if cmd == nil {
		t.Fatal("addUpdateCmd() returned nil")
	}

	if cmd.Use != "update <repository> ..." {
		t.Errorf("Expected Use to be 'update <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Update primary branch across repositories" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestUpdateCmdArgs(t *testing.T) {
	cmd := addUpdateCmd()

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

func TestUpdateCmdHelp(t *testing.T) {
	cmd := addUpdateCmd()

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
	if !strings.Contains(output, "Update primary branch") {
		t.Error("Help output should contain command description")
	}
}
