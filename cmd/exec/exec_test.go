package exec

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	testhelper "github.com/ryclarke/batch-tool/utils/testing"
)

func loadFixture(t *testing.T) context.Context {
	return testhelper.LoadFixture(t, "../../config")
}

// mockStdin replaces stdin with a mock for testing
func mockStdin(input string) io.Reader {
	return strings.NewReader(input)
}

func TestCmd(t *testing.T) {
	cmd := Cmd()

	if cmd == nil {
		t.Fatal("Cmd() returned nil")
	}

	if cmd.Use != "exec <repository>..." {
		t.Errorf("Expected Use to be 'exec <repository>...', got %s", cmd.Use)
	}

	// Test aliases
	aliases := cmd.Aliases
	if len(aliases) != 1 || aliases[0] != "sh" {
		t.Errorf("Expected aliases to contain 'sh', got %v", aliases)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}
}

func TestShellCmdFlags(t *testing.T) {
	cmd := Cmd()

	// Test exec flag
	cmdFlag := cmd.Flags().Lookup("script")
	if cmdFlag == nil {
		t.Fatal("cmd flag not found")
	}

	if cmdFlag.Shorthand != "c" {
		t.Errorf("Expected cmd flag shorthand to be 'c', got %s", cmdFlag.Shorthand)
	}

	if cmdFlag.Usage == "" {
		t.Error("Expected usage description to be set")
	}

	if cmdFlag.DefValue != "" {
		t.Errorf("Expected default value to be empty, got %s", cmdFlag.DefValue)
	}
}

func TestShellCmdArgs(t *testing.T) {
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

	err = cmd.Args(cmd, []string{"repo1", "repo2", "repo3"})
	if err != nil {
		t.Errorf("Expected no error with multiple arguments, got %v", err)
	}
}

func TestShellCmdWithoutExecFlag(t *testing.T) {
	cmd := Cmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Test with missing exec flag - should be empty string
	cmd.SetArgs([]string{"repo1"})

	// Get the exec flag value
	execValue, err := cmd.Flags().GetString("script")
	if err != nil {
		t.Errorf("Failed to get cmd flag value: %v", err)
	}

	if execValue != "" {
		t.Errorf("Expected exec value to be empty by default, got %s", execValue)
	}
}

func TestShellCmdWithExecFlag(t *testing.T) {
	cmd := Cmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Set the exec flag
	cmd.Flags().Set("script", "echo test")

	// Test that we can get the exec value
	execValue, err := cmd.Flags().GetString("script")
	if err != nil {
		t.Errorf("Failed to get cmd flag value: %v", err)
	}

	if execValue != "echo test" {
		t.Errorf("Expected exec value to be 'echo test', got %s", execValue)
	}
}

func TestShellCmdShorthandFlag(t *testing.T) {
	cmd := Cmd()

	// Test shorthand flag (-c)
	cmd.SetArgs([]string{"-c", "ls -la", "repo1"})

	// Parse the flags
	err := cmd.ParseFlags([]string{"-c", "ls -la"})
	if err != nil {
		t.Errorf("Failed to parse shorthand flag: %v", err)
	}

	execValue, err := cmd.Flags().GetString("script")
	if err != nil {
		t.Errorf("Failed to get cmd flag value: %v", err)
	}

	if execValue != "ls -la" {
		t.Errorf("Expected cmd value to be 'ls -la', got %s", execValue)
	}
}

func TestShellCmdRunE(t *testing.T) {
	cmd := Cmd()

	// Verify that RunE is set
	if cmd.RunE == nil {
		t.Error("Expected RunE function to be set")
	}
}

func TestShellCmdAliases(t *testing.T) {
	cmd := Cmd()

	// Verify aliases
	expectedAliases := []string{"sh"}
	if len(cmd.Aliases) != len(expectedAliases) {
		t.Errorf("Expected %d aliases, got %d", len(expectedAliases), len(cmd.Aliases))
	}

	for i, alias := range expectedAliases {
		if cmd.Aliases[i] != alias {
			t.Errorf("Expected alias %s at position %d, got %s", alias, i, cmd.Aliases[i])
		}
	}
}

func TestShellCmdDangerWarning(t *testing.T) {
	cmd := Cmd()

	// Verify the danger warning is in the short description
	if !strings.Contains(cmd.Short, "[!DANGEROUS!]") {
		t.Error("Short description should contain [!DANGEROUS!] warning")
	}

	if !strings.Contains(cmd.Short, "Execute a shell command") {
		t.Error("Short description should mention executing shell commands")
	}
}

func TestConfirmExecutionBasicResponses(t *testing.T) {
	exec := "echo test"
	args := []string{"repo1", "repo2"}

	tests := []struct {
		name      string
		input     string
		confirmed bool
	}{
		{"yes", "yes\n", true},
		{"y", "y\n", true},
		{"no", "no\n", false},
		{"n", "n\n", false},
		{"empty", "\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, exec, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if confirmed != tt.confirmed {
				t.Errorf("Expected confirmation to be %v for input '%s', got %v",
					tt.confirmed, strings.TrimSpace(tt.input), confirmed)
			}
		})
	}
}

func TestConfirmExecutionInvalidInput(t *testing.T) {
	exec := "echo test"
	args := []string{"repo1"}

	tests := []struct {
		name       string
		input      string
		confirmed  bool
		retryCount int
	}{
		{"invalid then yes", "invalid\nyes\n", true, 1},
		{"invalid then no", "maybe\nno\n", false, 1},
		{"multiple invalid then yes", "foo\nbar\nbaz\ny\n", true, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, exec, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if confirmed != tt.confirmed {
				t.Errorf("Expected confirmation to be %v, got %v", tt.confirmed, confirmed)
			}

			output := buf.String()
			if !strings.Contains(output, "Expected 'yes' ('y') or 'no' ('n')") {
				t.Error("Expected retry prompt after invalid input")
			}

			if tt.retryCount > 0 {
				promptCount := strings.Count(output, "Expected 'yes' ('y') or 'no' ('n')")
				if promptCount != tt.retryCount {
					t.Errorf("Expected %d retry prompts, got %d", tt.retryCount, promptCount)
				}
			}
		})
	}
}

func TestConfirmExecutionCaseInsensitive(t *testing.T) {
	exec := "echo test"
	args := []string{"repo1", "repo2"}

	tests := []struct {
		name      string
		input     string
		confirmed bool
	}{
		{"YES uppercase", "YES\n", true},
		{"Yes mixed case", "Yes\n", true},
		{"Y uppercase", "Y\n", true},
		{"NO uppercase", "NO\n", false},
		{"No mixed case", "No\n", false},
		{"N uppercase", "N\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, exec, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if confirmed != tt.confirmed {
				t.Errorf("Expected confirmation to be %v for input '%s', got %v",
					tt.confirmed, strings.TrimSpace(tt.input), confirmed)
			}
		})
	}
}

func TestConfirmExecutionWithWhitespace(t *testing.T) {
	exec := "ls -la"
	args := []string{"repo1"}

	tests := []struct {
		name      string
		input     string
		confirmed bool
	}{
		{"yes with spaces", "  yes  \n", true},
		{"y with tabs", "\ty\t\n", true},
		{"no with spaces", "  no  \n", false},
		{"n with tabs", "\tn\t\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, exec, args)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if confirmed != tt.confirmed {
				t.Errorf("Expected confirmation to be %v for input '%s', got %v",
					tt.confirmed, tt.input, confirmed)
			}
		})
	}
}

func TestShellCmdExecutionAborted(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(mockStdin("no\n"))

	cmd.SetArgs([]string{"-c", "echo test", "repo1"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Expected no error when user aborts, got: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Aborting.") {
		t.Error("Expected 'Aborting.' message when user says no")
	}
}

func TestShellCmdExecutionConfirmed(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(mockStdin("yes\n"))

	cmd.SetArgs([]string{"-c", "echo test", "repo1"})
	err := cmd.ExecuteContext(ctx)
	// May error if repo doesn't exist, but should get past confirmation
	// The important thing is it didn't abort

	output := buf.String()
	if strings.Contains(output, "Aborting.") {
		t.Error("Should not abort when user confirms")
	}

	if !strings.Contains(output, "Are you sure?") {
		t.Error("Expected confirmation prompt in output")
	}

	// We expect an error here because repo1 doesn't exist, but that's fine
	// We're just testing that we got past the confirmation stage
	_ = err
}

func TestShellCmdPromptFormat(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(mockStdin("n\n"))

	cmd.SetArgs([]string{"-c", "rm -rf /", "dangerous-repo"})
	err := cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Executing command: [dangerous-repo]") {
		t.Error("Expected to see command being executed in output")
	}

	if !strings.Contains(output, `sh -c "rm -rf /"`) {
		t.Error("Expected to see shell command in output")
	}

	if !strings.Contains(output, "Are you sure? [y/N]:") {
		t.Error("Expected to see confirmation prompt with default")
	}
}

func TestConfirmExecutionEOF(t *testing.T) {
	// Test handling of EOF (e.g., piped input that ends)
	exec := "echo test"
	args := []string{"repo1"}

	var buf bytes.Buffer
	_, err := confirmExecution(mockStdin(""), &buf, exec, args)
	if !errors.Is(err, io.EOF) {
		t.Errorf("Expected EOF error, got: %v", err)
	}
}

func TestConfirmExecutionPromptContent(t *testing.T) {
	tests := []struct {
		name     string
		exec     string
		args     []string
		wantExec string
		wantArgs string
	}{
		{
			name:     "single repo",
			exec:     "echo hello",
			args:     []string{"repo1"},
			wantExec: `sh -c "echo hello"`,
			wantArgs: "[repo1]",
		},
		{
			name:     "multiple repos",
			exec:     "git status",
			args:     []string{"repo1", "repo2", "repo3"},
			wantExec: `sh -c "git status"`,
			wantArgs: "[repo1 repo2 repo3]",
		},
		{
			name:     "dangerous command",
			exec:     "rm -rf /",
			args:     []string{"production"},
			wantExec: `sh -c "rm -rf /"`,
			wantArgs: "[production]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			_, _ = confirmExecution(mockStdin("n\n"), &buf, tt.exec, tt.args)

			output := buf.String()
			if !strings.Contains(output, tt.wantExec) {
				t.Errorf("Expected output to contain %q, got: %s", tt.wantExec, output)
			}

			if !strings.Contains(output, tt.wantArgs) {
				t.Errorf("Expected output to contain %q, got: %s", tt.wantArgs, output)
			}

			if !strings.Contains(output, "Executing command:") {
				t.Error("Expected output to contain 'Executing command:'")
			}

			if !strings.Contains(output, "Are you sure? [y/N]:") {
				t.Error("Expected output to contain confirmation prompt")
			}
		})
	}
}
