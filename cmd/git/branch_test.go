package git

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestAddBranchCmd(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addBranchCmd()

	if cmd == nil {
		t.Fatal("addBranchCmd() returned nil")
	}

	if cmd.Use != "branch <repository> ..." {
		t.Errorf("Expected Use to be 'branch <repository> ...', got %s", cmd.Use)
	}

	// Test aliases
	aliases := cmd.Aliases
	if len(aliases) != 1 || aliases[0] != "checkout" {
		t.Errorf("Expected aliases to contain 'checkout', got %v", aliases)
	}

	if cmd.Short != "Checkout a new branch across repositories" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestBranchCmdFlags(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addBranchCmd()

	// Test branch flag
	branchFlag := cmd.Flags().Lookup("branch")
	if branchFlag == nil {
		t.Error("branch flag not found")
	}

	if branchFlag.Shorthand != "b" {
		t.Errorf("Expected branch flag shorthand to be 'b', got %s", branchFlag.Shorthand)
	}
}

func TestBranchCmdArgs(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addBranchCmd()

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

func TestBranchCmdHelp(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addBranchCmd()

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
	if !strings.Contains(output, "Checkout a new branch") {
		t.Error("Help output should contain command description")
	}

	if !strings.Contains(output, "--branch") {
		t.Error("Help output should contain branch flag information")
	}
}

func TestBranchCmdPreRunE(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addBranchCmd()

	// Test PreRunE function exists
	if cmd.PreRunE == nil {
		t.Error("Expected PreRunE function to be set")
		return
	}

	// Test without branch flag (should require branch)
	err := cmd.PreRunE(cmd, []string{})
	if err == nil {
		t.Error("Expected error when branch flag is not set")
	}
}
