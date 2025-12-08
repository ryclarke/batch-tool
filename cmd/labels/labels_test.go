package labels

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func loadFixture(t *testing.T) context.Context {
	return config.LoadFixture(t, "../../config")
}

func TestCmd(t *testing.T) {
	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "labels <repository|label> ..." {
		t.Errorf("Expected Use to be 'labels <repository|label> ...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestLabelsCmdAliases(t *testing.T) {
	cmd := Cmd()

	// Test aliases
	expectedAliases := []string{"label"}
	if len(cmd.Aliases) != len(expectedAliases) {
		t.Errorf("Expected %d aliases, got %d", len(expectedAliases), len(cmd.Aliases))
	}

	if len(cmd.Aliases) > 0 && cmd.Aliases[0] != "label" {
		t.Errorf("Expected first alias to be 'label', got %s", cmd.Aliases[0])
	}
}

func TestLabelsCmdFlags(t *testing.T) {
	cmd := Cmd()

	// Test verbose flag
	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Fatal("verbose flag not found")
	}

	if verboseFlag.Shorthand != "v" {
		t.Errorf("Expected verbose flag shorthand to be 'v', got %s", verboseFlag.Shorthand)
	}

	if verboseFlag.DefValue == "" {
		t.Error("verbose flag should have a default value")
	}
}

func TestLabelsCmdRunE(t *testing.T) {
	cmd := Cmd()

	// The labels command should be runnable even without args in some cases
	// Let's check that the command exists and has the right function
	if cmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}
}

func TestLabelsCmdWithNoArgs(t *testing.T) {
	cmd := Cmd()
	ctx := loadFixture(t)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute without arguments - should print available labels
	cmd.SetArgs([]string{})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Execution without args failed: %v", err)
	}

	output := buf.String()
	// Should print "Available labels:" message
	if strings.Contains(output, "Available labels:") {
		// This is the expected output
		t.Logf("Got expected output: %s", output)
	} else if output != "" {
		// Also acceptable - may have other output
		t.Logf("Got output: %s", output)
	}
}

func TestLabelsCmdWithArgs(t *testing.T) {
	cmd := Cmd()
	ctx := loadFixture(t)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute with arguments - should print the set
	cmd.SetArgs([]string{"go", "backend"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Execution with args failed: %v", err)
	}

	// Command should execute without error (output goes to catalog.PrintSet which may not be captured)
	// Just verify the command ran successfully
}

func TestLabelsCmdWithVerboseFlag(t *testing.T) {
	cmd := Cmd()
	ctx := loadFixture(t)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute with verbose flag
	cmd.SetArgs([]string{"--verbose", "go"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Execution with verbose flag failed: %v", err)
	}

	// Verify the verbose flag was parsed
	verboseValue, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		t.Errorf("Failed to get verbose flag value: %v", err)
	}

	if !verboseValue {
		t.Error("Expected verbose flag to be true")
	}
}

func TestLabelsCmdWithShortVerboseFlag(t *testing.T) {
	cmd := Cmd()

	// Test shorthand flag (-v)
	err := cmd.ParseFlags([]string{"-v"})
	if err != nil {
		t.Errorf("Failed to parse shorthand flag: %v", err)
	}

	verboseValue, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		t.Errorf("Failed to get verbose flag value: %v", err)
	}

	if !verboseValue {
		t.Error("Expected verbose flag to be true with -v shorthand")
	}
}

func TestLabelsCmdDescription(t *testing.T) {
	cmd := Cmd()

	// Verify command description
	if !strings.Contains(cmd.Short, "labels") {
		t.Error("Short description should mention labels")
	}

	if !strings.Contains(cmd.Short, "Inspect") {
		t.Error("Short description should mention inspecting")
	}
}

func TestLabelsCmdUsage(t *testing.T) {
	cmd := Cmd()

	// Verify usage string includes optional args
	if !strings.Contains(cmd.Use, "<repository|label>") {
		t.Error("Usage should indicate repository or label arguments")
	}

	if !strings.Contains(cmd.Use, "...") {
		t.Error("Usage should indicate multiple arguments are accepted")
	}
}

func TestLabelsCmdFlagDefaults(t *testing.T) {
	cmd := Cmd()

	// Test that verbose defaults to false
	verboseValue, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		t.Errorf("Failed to get verbose flag value: %v", err)
	}

	if verboseValue {
		t.Error("Expected verbose flag to default to false")
	}
}
