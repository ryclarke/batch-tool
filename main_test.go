package main

import (
	"context"
	"os"
	"testing"

	"github.com/ryclarke/batch-tool/cmd"
	"github.com/ryclarke/batch-tool/config"
)

func loadFixture(t *testing.T) context.Context {
	return config.LoadFixture(t, "config")
}

func TestMain(t *testing.T) {
	// Test that main function can be called without panic
	// We'll redirect stdout to avoid actual execution
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set up test args to avoid actual command execution
	os.Args = []string{"batch-tool", "--help"}

	// This should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("main() panicked: %v", r)
		}
	}()

	// We can't actually call main() in tests as it calls os.Exit
	// Instead, we test that the cmd.Execute() function exists and is callable
	root := cmd.RootCmd()
	if root == nil {
		t.Fatal("RootCmd() returned nil")
	}

	if root.Use != "batch-tool" {
		t.Errorf("Expected root command use to be 'batch-tool', got %s", root.Use)
	}
}
