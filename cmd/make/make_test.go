package make

import (
	testhelper "github.com/ryclarke/batch-tool/utils/test"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/ryclarke/batch-tool/config"
	"github.com/spf13/cobra"
)

func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../../config")
}

// testCmd creates a test cobra command with the given context and output writer
func testCmd(ctx context.Context, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.SetOut(out)
	return cmd
}

func TestCmd(t *testing.T) {
	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "make <repository>..." {
		t.Errorf("Expected Use to be 'make <repository>...', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	if cmd.Long == "" {
		t.Error("Expected Long description to be set")
	}
}

func TestMakeCmdFlags(t *testing.T) {
	cmd := Cmd()

	// Test target flag
	targetFlag := cmd.Flags().Lookup("target")
	if targetFlag == nil {
		t.Error("target flag not found")
	}

	if targetFlag.Shorthand != "t" {
		t.Errorf("Expected target flag shorthand to be 't', got %s", targetFlag.Shorthand)
	}

	if targetFlag.DefValue == "" {
		t.Error("Expected default value to be set")
	}
}

func TestMakeCmdArgs(t *testing.T) {
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

	err = cmd.Args(cmd, []string{"repo1", "repo2"})
	if err != nil {
		t.Errorf("Expected no error with multiple arguments, got %v", err)
	}
}

func TestMakeCmdPreRunE(t *testing.T) {
	cmd := Cmd()

	// Test PreRunE function exists
	if cmd.PreRunE == nil {
		t.Error("Expected PreRunE function to be set")
		return
	}

	ctx := loadFixture(t)
	cmd.SetContext(ctx)

	// Test that PreRunE executes without error
	err := cmd.PreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("PreRunE should not return error, got: %v", err)
	}
}

func TestMakeCmdRun(t *testing.T) {
	cmd := Cmd()

	// Test Run function exists
	if cmd.Run == nil {
		t.Error("Expected Run function to be set")
		return
	}

	// Note: We can't easily test the actual execution without real repositories
	// but we can verify the function is set
}

func TestMakeCmdWithCustomTargets(t *testing.T) {
	cmd := Cmd()
	ctx := loadFixture(t)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	// Set custom targets
	cmd.Flags().Set("target", "build,test")

	// Execute PreRunE to bind flags
	err := cmd.PreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("PreRunE failed: %v", err)
	}

	// Verify targets were bound to viper
	viper := config.Viper(ctx)
	targets := viper.GetStringSlice(config.MakeTargets)

	if len(targets) != 2 {
		t.Errorf("Expected 2 targets, got %d", len(targets))
	}

	expectedTargets := []string{"build", "test"}
	for i, expected := range expectedTargets {
		if i >= len(targets) || targets[i] != expected {
			t.Errorf("Expected target[%d] to be '%s', got '%v'", i, expected, targets)
		}
	}
}

func TestMakeCmdDefaultTargets(t *testing.T) {
	cmd := Cmd()
	ctx := loadFixture(t)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	// Don't set any targets, should use default
	err := cmd.PreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("PreRunE failed: %v", err)
	}

	// Verify default targets were bound
	viper := config.Viper(ctx)
	targets := viper.GetStringSlice(config.MakeTargets)

	if len(targets) != 1 {
		t.Errorf("Expected 1 default target, got %d", len(targets))
	}

	if len(targets) > 0 && targets[0] != "format" {
		t.Errorf("Expected default target to be 'format', got '%s'", targets[0])
	}
}

func TestMakeCmdMultipleTargets(t *testing.T) {
	cmd := Cmd()
	ctx := loadFixture(t)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetContext(ctx)

	// Set multiple targets
	cmd.Flags().Set("target", "clean,build,test,lint")

	err := cmd.PreRunE(cmd, []string{})
	if err != nil {
		t.Errorf("PreRunE failed: %v", err)
	}

	viper := config.Viper(ctx)
	targets := viper.GetStringSlice(config.MakeTargets)

	expectedTargets := []string{"clean", "build", "test", "lint"}
	if len(targets) != len(expectedTargets) {
		t.Errorf("Expected %d targets, got %d", len(expectedTargets), len(targets))
	}

	for i, expected := range expectedTargets {
		if i >= len(targets) || targets[i] != expected {
			t.Errorf("Expected target[%d] to be '%s', got '%v'", i, expected, targets)
		}
	}
}

func TestMakeCmdValidArgsFunction(t *testing.T) {
	cmd := Cmd()

	// Test that ValidArgsFunction is set (for shell completion)
	if cmd.ValidArgsFunction == nil {
		t.Error("Expected ValidArgsFunction to be set for completion support")
	}
}

func TestMake(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	tests := []struct {
		name    string
		targets []string
		repo    string
		wantErr bool
	}{
		{
			name:    "single target",
			targets: []string{"format"},
			repo:    "test-repo",
			wantErr: true, // Expected to fail since repo doesn't exist
		},
		{
			name:    "multiple targets",
			targets: []string{"build", "test"},
			repo:    "test-repo",
			wantErr: true, // Expected to fail since repo doesn't exist
		},
		{
			name:    "empty targets",
			targets: []string{},
			repo:    "test-repo",
			wantErr: true, // Expected to fail since repo doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up targets in viper
			viper.Set(config.MakeTargets, tt.targets)

			ch := make(chan string, 10)
			err := Make(ctx, tt.repo, ch)
			close(ch)

			// In tests, we expect errors since repos don't exist
			if (err != nil) != tt.wantErr {
				t.Errorf("Make() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Collect output messages
			var messages []string
			for msg := range ch {
				messages = append(messages, msg)
			}

			// Verify we got some output (error messages)
			if len(messages) == 0 && err == nil {
				t.Error("Expected some output messages")
			}
		})
	}
}

func TestMakeWithEmptyTargets(t *testing.T) {
	ctx := loadFixture(t)
	viper := config.Viper(ctx)

	// Set empty targets
	viper.Set(config.MakeTargets, []string{})

	ch := make(chan string, 10)
	err := Make(ctx, "test-repo", ch)
	close(ch)

	// Should still attempt to run make (even without targets)
	// This will fail because repo doesn't exist
	if err == nil {
		t.Error("Expected error when running make on non-existent repo")
	}
}
