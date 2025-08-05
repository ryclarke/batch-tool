package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCmd(t *testing.T) {
	cmd := RootCmd()

	if cmd == nil {
		t.Fatal("RootCmd() returned nil")
	}

	if cmd.Use != "batch-tool" {
		t.Errorf("Expected Use to be 'batch-tool', got %s", cmd.Use)
	}

	if cmd.Short != "Batch tool for working across multiple git repositories" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}

	// Test that subcommands are added
	subcommands := cmd.Commands()
	expectedCommands := []string{"version", "catalog", "git", "pr", "make", "shell", "labels"}

	if len(subcommands) < len(expectedCommands) {
		t.Errorf("Expected at least %d subcommands, got %d", len(expectedCommands), len(subcommands))
	}
}

func TestVersionCommand(t *testing.T) {
	cmd := RootCmd()

	// Find the version command
	var versionCmd *cobra.Command
	for _, subcmd := range cmd.Commands() {
		if subcmd.Use == "version" {
			versionCmd = subcmd
			break
		}
	}

	if versionCmd == nil {
		t.Fatal("version command not found")
	}

	if versionCmd.Short != "Print the current batch-tool version" {
		t.Errorf("Expected correct version command description, got %s", versionCmd.Short)
	}
}

func TestCatalogCommand(t *testing.T) {
	cmd := RootCmd()

	// Find the catalog command
	var catalogCmd *cobra.Command
	for _, subcmd := range cmd.Commands() {
		if subcmd.Use == "catalog" {
			catalogCmd = subcmd
			break
		}
	}

	if catalogCmd == nil {
		t.Fatal("catalog command not found")
	}

	if catalogCmd.Short != "Print information on the cached repository catalog" {
		t.Errorf("Expected correct catalog command description, got %s", catalogCmd.Short)
	}
}

func TestPersistentFlags(t *testing.T) {
	cmd := RootCmd()

	// Test that persistent flags are set up
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Error("config flag not found")
	}

	syncFlag := cmd.PersistentFlags().Lookup("sync")
	if syncFlag == nil {
		t.Error("sync flag not found")
	}

	sortFlag := cmd.PersistentFlags().Lookup("sort")
	if sortFlag == nil {
		t.Error("sort flag not found")
	}

	skipUnwantedFlag := cmd.PersistentFlags().Lookup("skip-unwanted")
	if skipUnwantedFlag == nil {
		t.Error("skip-unwanted flag not found")
	}
}

func TestPersistentPreRun(t *testing.T) {
	// Test the persistent pre-run logic
	cmd := RootCmd()

	// Create a buffer to capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test no-sort flag behavior - should be available even if no PersistentPreRun
	cmd.PersistentFlags().Set("no-sort", "true")

	// Since there's no PersistentPreRun function, just verify the flags exist
	if cmd.PersistentPreRun == nil {
		t.Log("No PersistentPreRun function - flags will be processed during command execution")
	}

	// Test no-skip-unwanted flag behavior
	cmd.PersistentFlags().Set("no-skip-unwanted", "true")

	// Verify the flags were set
	if flag := cmd.PersistentFlags().Lookup("no-sort"); flag == nil {
		t.Error("no-sort flag should exist")
	}
	if flag := cmd.PersistentFlags().Lookup("no-skip-unwanted"); flag == nil {
		t.Error("no-skip-unwanted flag should exist")
	}
}

func TestExecuteWithHelp(t *testing.T) {
	// Test that Execute function can handle help flag without error
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"batch-tool", "--help"}

	// Create a buffer to capture output
	var buf bytes.Buffer
	cmd := RootCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute with help flag should not cause error
	err := cmd.Execute()
	if err != nil {
		t.Errorf("Execute with --help flag failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Batch tool for working across multiple git repositories") {
		t.Error("Help output should contain the application description")
	}
}

func TestHiddenFlags(t *testing.T) {
	cmd := RootCmd()

	// Test that hidden flags are properly configured
	noSortFlag := cmd.PersistentFlags().Lookup("no-sort")
	if noSortFlag == nil {
		t.Error("no-sort flag not found")
	} else if !noSortFlag.Hidden {
		t.Error("no-sort flag should be hidden")
	}

	noSkipUnwantedFlag := cmd.PersistentFlags().Lookup("no-skip-unwanted")
	if noSkipUnwantedFlag == nil {
		t.Error("no-skip-unwanted flag not found")
	} else if !noSkipUnwantedFlag.Hidden {
		t.Error("no-skip-unwanted flag should be hidden")
	}
}
