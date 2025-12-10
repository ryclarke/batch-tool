package git

import (
	"context"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func loadFixture(t *testing.T) context.Context {
	return config.LoadFixture(t, "../../config")
}

func TestGitCmd(t *testing.T) {
	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "git [cmd] <repository>..." {
		t.Errorf("Expected Use to be 'git [cmd] <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestGitCmdSubcommands(t *testing.T) {
	cmd := Cmd()

	subcommands := cmd.Commands()
	expectedCommands := []string{"branch", "commit", "diff", "status", "update"}

	if len(subcommands) < len(expectedCommands) {
		t.Errorf("Expected at least %d subcommands, got %d", len(expectedCommands), len(subcommands))
	}

	// Check that expected subcommands exist
	commandNames := make(map[string]bool)
	for _, subcmd := range subcommands {
		commandNames[subcmd.Name()] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !commandNames[expectedCmd] {
			t.Errorf("Expected subcommand %s not found", expectedCmd)
		}
	}
}

func TestAddStatusCmd(t *testing.T) {
	cmd := addStatusCmd()

	if cmd == nil {
		t.Fatal("addStatusCmd() returned nil")
	}

	if cmd.Use != "status <repository>..." {
		t.Errorf("Expected Use to be 'status <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
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

func TestAddDiffCmd(t *testing.T) {
	cmd := addDiffCmd()

	if cmd == nil {
		t.Fatal("addDiffCmd() returned nil")
	}

	if cmd.Use != "diff <repository>..." {
		t.Errorf("Expected Use to be 'diff <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
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

func TestAddUpdateCmd(t *testing.T) {
	cmd := addUpdateCmd()

	if cmd == nil {
		t.Fatal("addUpdateCmd() returned nil")
	}

	if cmd.Use != "update <repository>..." {
		t.Errorf("Expected Use to be 'update <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
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
