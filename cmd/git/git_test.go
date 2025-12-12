package git

import (
	"testing"
)

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
	expectedCommands := []string{"status", "branch", "commit", "diff", "push", "update"}

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
