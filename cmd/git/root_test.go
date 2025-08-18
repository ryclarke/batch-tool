package git

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestGitCmd(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "git [cmd] <repository> ..." {
		t.Errorf("Expected Use to be 'git [cmd] <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Manage git branches and commits" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestGitCmdSubcommands(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := Cmd()

	subcommands := cmd.Commands()
	expectedCommands := []string{"checkout", "commit", "diff", "status", "update"}

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

func TestGitCmdHelp(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := Cmd()

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
	if !strings.Contains(output, "Manage git branches and commits") {
		t.Error("Help output should contain command description")
	}

	if !strings.Contains(output, "Available Commands:") {
		t.Error("Help output should show available subcommands")
	}
}
