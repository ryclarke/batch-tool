package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddLabelsCmd(t *testing.T) {
	cmd := addLabelsCmd()

	if cmd == nil {
		t.Fatal("addLabelsCmd() returned nil")
	}

	if cmd.Use != "labels <repository|label> ..." {
		t.Errorf("Expected Use to be 'labels <repository|label> ...', got %s", cmd.Use)
	}

	expectedShort := "Inspect repository labels and test filters"
	if cmd.Short != expectedShort {
		t.Errorf("Expected correct Short description, got %s", cmd.Short)
	}
}

func TestLabelsCmdArgs(t *testing.T) {
	cmd := addLabelsCmd()

	// The labels command should be runnable even without args in some cases
	// Let's check that the command exists and has the right function
	if cmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}
}

func TestLabelsCmdHelp(t *testing.T) {
	cmd := addLabelsCmd()

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
	if !strings.Contains(output, "Inspect repository labels and test filters") {
		t.Error("Help output should contain command description")
	}
}
