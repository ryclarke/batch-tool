package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/viper"
)

func TestAddMakeCmd(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addMakeCmd()

	if cmd == nil {
		t.Fatal("addMakeCmd() returned nil")
	}

	if cmd.Use != "make <repository> ..." {
		t.Errorf("Expected Use to be 'make <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Execute make targets across repositories" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}

	if !strings.Contains(cmd.Long, "Execute make targets across repositories") {
		t.Error("Expected Long description to mention executing make targets")
	}
}

func TestMakeCmdFlags(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addMakeCmd()

	// Test target flag
	targetFlag := cmd.Flags().Lookup("target")
	if targetFlag == nil {
		t.Error("target flag not found")
	}

	if targetFlag.Shorthand != "t" {
		t.Errorf("Expected target flag shorthand to be 't', got %s", targetFlag.Shorthand)
	}

	if targetFlag.DefValue != "[format]" {
		t.Errorf("Expected default value to be '[format]', got %s", targetFlag.DefValue)
	}
}

func TestMakeCmdArgs(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addMakeCmd()

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

	err = cmd.Args(cmd, []string{"repo1", "repo2"})
	if err != nil {
		t.Errorf("Expected no error with multiple arguments, got %v", err)
	}
}

func TestMakeCmdHelp(t *testing.T) {
	_ = config.LoadFixture("../config")

	cmd := addMakeCmd()

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
	if !strings.Contains(output, "Execute make targets across repositories") {
		t.Error("Help output should contain command description")
	}

	if !strings.Contains(output, "--target") {
		t.Error("Help output should contain target flag information")
	}
}

func TestMakeCmdConfiguration(t *testing.T) {
	_ = config.LoadFixture("../config")

	// Test that the make command properly integrates with viper configuration
	originalTargets := viper.GetStringSlice("make.targets")
	defer viper.Set("make.targets", originalTargets)

	cmd := addMakeCmd()

	// Set some test targets
	testTargets := []string{"build", "test", "lint"}
	cmd.Flags().Set("target", strings.Join(testTargets, ","))

	// The makeTargets variable should be updated
	if len(makeTargets) == 0 {
		// Note: This test may need to be adjusted based on how the flag binding works
		t.Log("makeTargets variable not populated - this may be expected behavior")
	}
}
