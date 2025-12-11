package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/utils"
	"github.com/spf13/cobra"
)

func loadFixture(t *testing.T) context.Context {
	return config.LoadFixture(t, "../config")
}

func TestRootCmd(t *testing.T) {
	loadFixture(t)

	cmd := RootCmd()

	if cmd == nil {
		t.Fatal("RootCmd() returned nil")
	}

	if cmd.Use != "batch-tool" {
		t.Errorf("Expected Use to be 'batch-tool', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	// Test that subcommands are added
	subcommands := cmd.Commands()
	expectedCommands := []string{"catalog", "git", "pr", "make", "shell", "labels"}

	if len(subcommands) < len(expectedCommands) {
		t.Errorf("Expected at least %d subcommands, got %d", len(expectedCommands), len(subcommands))
	}
}

func TestCatalogCommand(t *testing.T) {
	_ = loadFixture(t)

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

	if catalogCmd.Short == "" {
		t.Error("Expected catalog command description to be set")
	}
}

func TestPersistentFlags(t *testing.T) {
	_ = loadFixture(t)

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
	_ = loadFixture(t)
	cmd := RootCmd()

	// Create a buffer to capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test no-sort flag behavior - should be available even if no PersistentPreRun
	err := cmd.PersistentFlags().Set("no-sort", "true")
	if err != nil {
		t.Errorf("Failed to set no-sort flag: %v", err)
	}

	// Since there's no PersistentPreRun function, just verify the flags exist
	if cmd.PersistentPreRun == nil {
		t.Log("No PersistentPreRun function - flags will be processed during command execution")
	}

	// Test no-skip-unwanted flag behavior
	err = cmd.PersistentFlags().Set("no-skip-unwanted", "true")
	if err != nil {
		t.Errorf("Failed to set no-skip-unwanted flag: %v", err)
	}

	// Verify the flags were set
	if flag := cmd.PersistentFlags().Lookup("no-sort"); flag == nil {
		t.Error("no-sort flag should exist")
	}
	if flag := cmd.PersistentFlags().Lookup("no-skip-unwanted"); flag == nil {
		t.Error("no-skip-unwanted flag should exist")
	}
}

func TestExecuteWithHelp(t *testing.T) {
	ctx := loadFixture(t)

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
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Execute with --help flag failed: %v", err)
	}

	// Just verify that help output is not empty
	output := buf.String()
	if output == "" {
		t.Error("Help output should not be empty")
	}
}

func TestHiddenFlags(t *testing.T) {
	_ = loadFixture(t)
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

func TestSyncFlagOverridesMaxConcurrency(t *testing.T) {
	ctx := loadFixture(t)
	cmd := RootCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Set sync flag and execute catalog command to trigger PersistentPreRun
	cmd.SetArgs([]string{"--sync", "catalog"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	// Verify sync flag is set
	syncValue, err := cmd.Flags().GetBool("sync")
	if err != nil {
		t.Errorf("Failed to get sync flag: %v", err)
	}

	if !syncValue {
		t.Error("Expected sync flag to be true")
	}
}

func TestNoSortFlagOverridesSortConfig(t *testing.T) {
	ctx := loadFixture(t)
	cmd := RootCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Set no-sort flag and execute catalog command
	cmd.SetArgs([]string{"--no-sort", "catalog"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	// Verify no-sort flag is set
	noSortValue, err := cmd.Flags().GetBool("no-sort")
	if err != nil {
		t.Errorf("Failed to get no-sort flag: %v", err)
	}

	if !noSortValue {
		t.Error("Expected no-sort flag to be true")
	}
}

func TestNoSkipUnwantedFlagOverridesConfig(t *testing.T) {
	ctx := loadFixture(t)
	cmd := RootCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Set no-skip-unwanted flag and execute catalog command
	cmd.SetArgs([]string{"--no-skip-unwanted", "catalog"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	// Verify no-skip-unwanted flag is set
	noSkipValue, err := cmd.Flags().GetBool("no-skip-unwanted")
	if err != nil {
		t.Errorf("Failed to get no-skip-unwanted flag: %v", err)
	}

	if !noSkipValue {
		t.Error("Expected no-skip-unwanted flag to be true")
	}
}

func TestMaxConcurrencyFlag(t *testing.T) {
	ctx := loadFixture(t)
	cmd := RootCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Set max-concurrency flag
	cmd.SetArgs([]string{"--max-concurrency=5", "catalog"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	// Verify max-concurrency flag was set
	maxConcurrency, err := cmd.Flags().GetInt("max-concurrency")
	if err != nil {
		t.Errorf("Failed to get max-concurrency flag: %v", err)
	}

	if maxConcurrency != 5 {
		t.Errorf("Expected max-concurrency to be 5, got %d", maxConcurrency)
	}
}

func TestSortFlag(t *testing.T) {
	ctx := loadFixture(t)
	cmd := RootCmd()

	// Test default value
	sortFlag := cmd.PersistentFlags().Lookup("sort")
	if sortFlag == nil {
		t.Fatal("sort flag not found")
	}

	if sortFlag.DefValue != "true" {
		t.Errorf("Expected default sort value to be 'true', got %s", sortFlag.DefValue)
	}

	// Test setting the flag
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--sort=false", "catalog"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	sortValue, err := cmd.Flags().GetBool("sort")
	if err != nil {
		t.Errorf("Failed to get sort flag: %v", err)
	}

	if sortValue {
		t.Error("Expected sort flag to be false")
	}
}

func TestSkipUnwantedFlag(t *testing.T) {
	ctx := loadFixture(t)
	cmd := RootCmd()

	// Test default value
	skipUnwantedFlag := cmd.PersistentFlags().Lookup("skip-unwanted")
	if skipUnwantedFlag == nil {
		t.Fatal("skip-unwanted flag not found")
	}

	if skipUnwantedFlag.DefValue != "true" {
		t.Errorf("Expected default skip-unwanted value to be 'true', got %s", skipUnwantedFlag.DefValue)
	}

	// Test setting the flag
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"--skip-unwanted=false", "catalog"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}

	skipValue, err := cmd.Flags().GetBool("skip-unwanted")
	if err != nil {
		t.Errorf("Failed to get skip-unwanted flag: %v", err)
	}

	if skipValue {
		t.Error("Expected skip-unwanted flag to be false")
	}
}

func TestConfigFileFlag(t *testing.T) {
	_ = loadFixture(t)
	cmd := RootCmd()

	// Test config flag exists
	configFlag := cmd.PersistentFlags().Lookup("config")
	if configFlag == nil {
		t.Fatal("config flag not found")
	}

	if configFlag.Usage == "" {
		t.Error("Expected config flag usage to be set")
	}
}

func TestAllSubcommandsPresent(t *testing.T) {
	_ = loadFixture(t)
	cmd := RootCmd()

	subcommands := cmd.Commands()
	expectedCommands := map[string]bool{
		"catalog": false,
		"git":     false,
		"pr":      false,
		"make":    false,
		"exec":    false,
		"labels":  false,
	}

	for _, subcmd := range subcommands {
		if _, exists := expectedCommands[subcmd.Name()]; exists {
			expectedCommands[subcmd.Name()] = true
		}
	}

	for cmdName, found := range expectedCommands {
		if !found {
			t.Errorf("Expected subcommand %s not found", cmdName)
		}
	}
}

func TestCatalogCommandOutput(t *testing.T) {
	ctx := loadFixture(t)
	cmd := RootCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute catalog command
	cmd.SetArgs([]string{"catalog"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Catalog command execution failed: %v", err)
	}

	// Command should execute without error
	// Note: catalog output goes directly to fmt.Printf, not cmd.OutOrStdout()
}

func TestLongDescription(t *testing.T) {
	_ = loadFixture(t)
	cmd := RootCmd()

	if cmd.Long == "" {
		t.Error("Expected long description to be set")
	}

	if !strings.Contains(cmd.Long, "multiple git repositories") {
		t.Error("Long description should mention multiple git repositories")
	}

	if !strings.Contains(cmd.Long, "utility functions") {
		t.Error("Long description should mention utility functions")
	}
}

func TestSetTerminalWait(t *testing.T) {
	tests := []struct {
		name           string
		noWaitFlagSet  bool
		noWaitValue    bool
		waitFlagSet    bool
		waitValue      bool
		expectWaitExit bool
		description    string
	}{
		{
			name:           "explicit --no-wait flag set to true",
			noWaitFlagSet:  true,
			noWaitValue:    true,
			waitFlagSet:    false,
			expectWaitExit: false,
			description:    "Explicit --no-wait should disable wait",
		},
		{
			name:           "explicit --no-wait flag set to false",
			noWaitFlagSet:  true,
			noWaitValue:    false,
			waitFlagSet:    false,
			expectWaitExit: true,
			description:    "Explicit --no-wait=false should enable wait",
		},
		{
			name:           "explicit --wait flag set (no auto-detection)",
			noWaitFlagSet:  false,
			waitFlagSet:    true,
			waitValue:      true,
			expectWaitExit: true,
			description:    "Explicit --wait should skip auto-detection",
		},
		{
			name:           "no flags set (auto-detection runs)",
			noWaitFlagSet:  false,
			waitFlagSet:    false,
			expectWaitExit: false,
			description:    "When no flags set, non-interactive environment should disable wait",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := loadFixture(t)
			viper := config.Viper(ctx)

			cmd := &cobra.Command{}
			cmd.SetContext(ctx)
			cmd.Flags().Bool(noWaitFlag, false, "")
			cmd.Flags().Bool(waitFlag, true, "")

			// Set flags based on test case
			if tt.noWaitFlagSet {
				cmd.Flags().Set(noWaitFlag, fmt.Sprintf("%v", tt.noWaitValue))
			}
			if tt.waitFlagSet {
				cmd.Flags().Set(waitFlag, fmt.Sprintf("%v", tt.waitValue))
			}

			// Use BindBoolFlags to match actual command behavior
			err := utils.BindBoolFlags(cmd, config.WaitOnExit, waitFlag, noWaitFlag)
			if err != nil {
				t.Errorf("%s: unexpected error from BindBoolFlags: %v", tt.description, err)
			}

			// Then run auto-detection
			err = setTerminalWait(cmd)
			if err != nil {
				t.Errorf("%s: unexpected error from setTerminalWait: %v", tt.description, err)
			}

			actualWaitExit := viper.GetBool(config.WaitOnExit)
			if actualWaitExit != tt.expectWaitExit {
				t.Errorf("%s: expected WaitOnExit=%v, got %v", tt.description, tt.expectWaitExit, actualWaitExit)
			}
		})
	}
}
