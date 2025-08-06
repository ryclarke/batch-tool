package pr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ryclarke/batch-tool/config"
)

func TestAddNewCmd(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addNewCmd()

	if cmd == nil {
		t.Fatal("addNewCmd() returned nil")
	}

	if cmd.Use != "new <repository> ..." {
		t.Errorf("Expected Use to be 'new <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Submit new pull requests" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestNewCmdFlags(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addNewCmd()

	// Test all-reviewers flag
	allReviewersFlag := cmd.Flags().Lookup("all-reviewers")
	if allReviewersFlag == nil {
		t.Error("all-reviewers flag not found")

		return
	}

	if allReviewersFlag.Shorthand != "a" {
		t.Errorf("Expected all-reviewers flag shorthand to be 'a', got %s", allReviewersFlag.Shorthand)
	}
}

func TestNewCmdArgs(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addNewCmd()

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

func TestNewCmdHelp(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addNewCmd()

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
	if !strings.Contains(output, "Submit new pull requests") {
		t.Error("Help output should contain command description")
	}

	if !strings.Contains(output, "--all-reviewers") {
		t.Error("Help output should contain all-reviewers flag information")
	}
}

func TestAddEditCmd(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addEditCmd()

	if cmd == nil {
		t.Fatal("addEditCmd() returned nil")
	}

	if cmd.Use != "edit <repository> ..." {
		t.Errorf("Expected Use to be 'edit <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Update existing pull requests" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestEditCmdFlags(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addEditCmd()

	// Test no-append flag
	noAppendFlag := cmd.Flags().Lookup("no-append")
	if noAppendFlag == nil {
		t.Error("no-append flag not found")
	}
}

func TestEditCmdArgs(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addEditCmd()

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

func TestEditCmdHelp(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addEditCmd()

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
	if !strings.Contains(output, "Update existing pull requests") {
		t.Error("Help output should contain command description")
	}

	if !strings.Contains(output, "--no-append") {
		t.Error("Help output should contain no-append flag information")
	}
}

func TestAddMergeCmd(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addMergeCmd()

	if cmd == nil {
		t.Fatal("addMergeCmd() returned nil")
	}

	if cmd.Use != "merge <repository> ..." {
		t.Errorf("Expected Use to be 'merge <repository> ...', got %s", cmd.Use)
	}

	if cmd.Short != "Merge accepted pull requests" {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestMergeCmdArgs(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addMergeCmd()

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

func TestMergeCmdHelp(t *testing.T) {
	_ = config.LoadFixture("../../config")

	cmd := addMergeCmd()

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
	if !strings.Contains(output, "Merge accepted pull requests") {
		t.Error("Help output should contain command description")
	}
}
