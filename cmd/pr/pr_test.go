package pr

import (
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestPrCmd(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "pr <repository>..." {
		t.Errorf("Expected Use to be 'pr <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestPrCmdSubcommands(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	subcommands := cmd.Commands()
	expectedCommands := []string{"new", "edit", "merge"}

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

func TestPrCmdFlags(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

	// Test persistent flags
	titleFlag := cmd.PersistentFlags().Lookup("title")
	if titleFlag == nil {
		t.Error("title flag not found")

		return
	}

	if titleFlag.Shorthand != "t" {
		t.Errorf("Expected title flag shorthand to be 't', got %s", titleFlag.Shorthand)
	}

	descFlag := cmd.PersistentFlags().Lookup("description")
	if descFlag == nil {
		t.Error("description flag not found")

		return
	}

	if descFlag.Shorthand != "d" {
		t.Errorf("Expected description flag shorthand to be 'd', got %s", descFlag.Shorthand)
	}

	reviewerFlag := cmd.PersistentFlags().Lookup("reviewer")
	if reviewerFlag == nil {
		t.Error("reviewer flag not found")

		return
	}

	if reviewerFlag.Shorthand != "r" {
		t.Errorf("Expected reviewer flag shorthand to be 'r', got %s", reviewerFlag.Shorthand)
	}
}

func TestPrCmdArgs(t *testing.T) {
	loadFixture(t)

	cmd := Cmd()

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

func TestPrCmdPersistentPreRunE(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	cmd := Cmd()
	cmd.SetContext(ctx)

	// Test PersistentPreRunE function exists
	if cmd.PersistentPreRunE == nil {
		t.Error("Expected PersistentPreRunE function to be set")

		return
	}

	// Test without auth token (should require auth token)
	viper.Set(config.AuthToken, "")
	err := cmd.PersistentPreRunE(cmd, []string{})
	if err == nil {
		t.Error("Expected error when auth token is not set")
	}
}
