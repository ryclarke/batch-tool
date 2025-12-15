package git

import (
	"testing"
)

func TestAddBranchCmd(t *testing.T) {
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

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestBranchCmdFlags(t *testing.T) {

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

func TestBranchCmdPreRunE(t *testing.T) {
	ctx := loadFixture(t)
	cmd := addBranchCmd()
	cmd.SetContext(ctx)

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
