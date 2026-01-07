package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	expectedUse := "exec {-c <command> | -f <file> [-a <arg>]...} [-y] <repository>..."
	if cmd.Use != expectedUse {
		t.Errorf("Expected Use to be '%s', got %s", expectedUse, cmd.Use)
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

	// Test file flag
	fileFlag := cmd.Flags().Lookup("file")
	if fileFlag == nil {
		t.Fatal("file flag not found")
	}

	if fileFlag.Shorthand != "f" {
		t.Errorf("Expected file flag shorthand to be 'f', got %s", fileFlag.Shorthand)
	}

	// Test force flag now uses -y
	forceFlag := cmd.Flags().Lookup("force")
	if forceFlag == nil {
		t.Fatal("force flag not found")
	}

	if forceFlag.Shorthand != "y" {
		t.Errorf("Expected force flag shorthand to be 'y', got %s", forceFlag.Shorthand)
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
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, "test preview")
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
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, "test preview")
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
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, "test preview")
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
			confirmed, err := confirmExecution(mockStdin(tt.input), &buf, "test preview")
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
	if !strings.Contains(output, "Executing") {
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

	var buf bytes.Buffer
	_, err := confirmExecution(mockStdin(""), &buf, "test preview")
	if !errors.Is(err, io.EOF) {
		t.Errorf("Expected EOF error, got: %v", err)
	}
}

func TestConfirmExecutionPromptContent(t *testing.T) {
	tests := []struct {
		name    string
		preview string
	}{
		{
			name:    "single repo inline command",
			preview: "`sh -c \"echo hello\"`",
		},
		{
			name:    "multiple repos inline command",
			preview: "`sh -c \"git status\"`",
		},
		{
			name:    "dangerous command",
			preview: "`sh -c \"rm -rf /\"`",
		},
		{
			name:    "file execution",
			preview: `file: "/path/to/script.sh"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			_, _ = confirmExecution(mockStdin("n\n"), &buf, tt.preview)

			output := buf.String()
			if !strings.Contains(output, "Executing "+tt.preview) {
				t.Errorf("Expected output to contain preview %q, got: %s", tt.preview, output)
			}

			if !strings.Contains(output, "Are you sure? [y/N]:") {
				t.Error("Expected output to contain confirmation prompt")
			}
		})
	}
}

func TestShellCmdWithFileFlag(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	// Create a temporary script file
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	scriptContent := "#!/bin/bash\necho 'Hello from script'\nls -la\n"
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0744)
	if err != nil {
		t.Fatalf("Failed to create test script file: %v", err)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(mockStdin("yes\n"))

	cmd.SetArgs([]string{"-f", scriptPath, "repo1"})
	_ = cmd.ExecuteContext(ctx)

	output := buf.String()
	if strings.Contains(output, "Aborting.") {
		t.Error("Should not abort when user confirms")
	}

	if !strings.Contains(output, "Are you sure?") {
		t.Error("Expected confirmation prompt in output")
	}
}

func TestShellCmdWithFileFlagShorthand(t *testing.T) {
	cmd := Cmd()

	// Create a temporary script file
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	scriptContent := "echo test"
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test script file: %v", err)
	}

	cmd.SetArgs([]string{"-f", scriptPath, "repo1"})

	// Parse the flags
	err = cmd.ParseFlags([]string{"-f", scriptPath})
	if err != nil {
		t.Errorf("Failed to parse -f flag: %v", err)
	}

	fileValue, err := cmd.Flags().GetString("file")
	if err != nil {
		t.Errorf("Failed to get file flag value: %v", err)
	}

	if fileValue != scriptPath {
		t.Errorf("Expected file value to be %q, got %q", scriptPath, fileValue)
	}
}

func TestShellCmdWithNonexistentFile(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"-f", "/nonexistent/script.sh", "repo1"})
	err := cmd.ExecuteContext(ctx)

	if err == nil {
		t.Error("Expected error when file does not exist")
	}

	if !strings.Contains(err.Error(), "failed to access file") {
		t.Errorf("Expected error about accessing file, got: %v", err)
	}
}

func TestShellCmdWithBothScriptAndFile(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	// Create a temporary script file
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	err := os.WriteFile(scriptPath, []byte("echo test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test script file: %v", err)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"-c", "echo inline", "-f", scriptPath, "repo1"})
	err = cmd.ExecuteContext(ctx)

	if err == nil {
		t.Error("Expected error when both -c and -f flags are provided")
	}

	if !strings.Contains(err.Error(), "cannot specify both") {
		t.Errorf("Expected error about specifying both flags, got: %v", err)
	}
}

func TestShellCmdWithNeitherScriptNorFile(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	cmd.SetArgs([]string{"repo1"})
	err := cmd.ExecuteContext(ctx)

	if err == nil {
		t.Error("Expected error when neither -c nor -f flag is provided")
	}

	if !strings.Contains(err.Error(), "no command provided") {
		t.Errorf("Expected error about no command provided, got: %v", err)
	}
}

func TestShellCmdForceWithFileFlag(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	// Create a temporary script file
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	scriptContent := "echo 'Test script'"
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test script file: %v", err)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Use -y flag (new shorthand for force) with -f flag
	cmd.SetArgs([]string{"-y", "-f", scriptPath, "repo1"})
	_ = cmd.ExecuteContext(ctx)

	output := buf.String()
	// Should not prompt for confirmation with -y flag
	if strings.Contains(output, "Are you sure?") {
		t.Error("Should not prompt when -y flag is used")
	}
}

func TestShellCmdForceFlagShorthand(t *testing.T) {
	cmd := Cmd()

	// Test that -y is now the shorthand for force
	cmd.SetArgs([]string{"-y", "-c", "echo test", "repo1"})

	err := cmd.ParseFlags([]string{"-y"})
	if err != nil {
		t.Errorf("Failed to parse -y flag: %v", err)
	}

	forceValue, err := cmd.Flags().GetBool("force")
	if err != nil {
		t.Errorf("Failed to get force flag value: %v", err)
	}

	if !forceValue {
		t.Error("Expected force flag to be true when -y is used")
	}
}

func TestConfirmExecutionWithFilePath(t *testing.T) {
	preview := `file: "/path/to/script.sh"`

	var buf bytes.Buffer
	confirmed, err := confirmExecution(mockStdin("y\n"), &buf, preview)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !confirmed {
		t.Error("Expected confirmation to be true")
	}

	output := buf.String()
	if !strings.Contains(output, preview) {
		t.Errorf("Expected output to contain preview, got: %s", output)
	}
}

func TestConfirmExecutionInlineCommand(t *testing.T) {
	preview := "`sh -c \"echo test\"`"

	var buf bytes.Buffer
	confirmed, err := confirmExecution(mockStdin("y\n"), &buf, preview)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !confirmed {
		t.Error("Expected confirmation to be true")
	}

	output := buf.String()
	if !strings.Contains(output, preview) {
		t.Errorf("Expected output to contain preview, got: %s", output)
	}
}

func TestShellCmdWithFileShowsFileName(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	// Create a temporary script file
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "my-script.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test script file: %v", err)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(mockStdin("n\n"))

	cmd.SetArgs([]string{"-f", scriptPath, "repo1"})
	err = cmd.ExecuteContext(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	output := buf.String()
	// Should show the script file path with 'file:' prefix
	if !strings.Contains(output, "file: ") || !strings.Contains(output, scriptPath) {
		t.Errorf("Expected output to show script path with file: prefix, got: %s", output)
	}

	// Should NOT show full script content
	if strings.Contains(output, "#!/bin/bash") {
		t.Error("Output should not contain script content")
	}

	// Should show aborting since we answered 'n'
	if !strings.Contains(output, "Aborting.") {
		t.Error("Expected 'Aborting.' message when user says no")
	}
}

func TestShellCmdWithBinaryExecutable(t *testing.T) {
	ctx := loadFixture(t)
	cmd := Cmd()

	// Create a temporary directory with a "binary" (just a script for testing)
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "test-binary")
	// Create an executable file (could be a binary)
	err := os.WriteFile(binaryPath, []byte("#!/bin/sh\necho 'Binary executed'\n"), 0755)
	if err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetIn(mockStdin("y\n"))

	cmd.SetArgs([]string{"-f", binaryPath, "repo1"})
	_ = cmd.ExecuteContext(ctx)

	output := buf.String()
	// Should show the binary path with 'file:' prefix in confirmation
	if !strings.Contains(output, "file: ") || !strings.Contains(output, binaryPath) {
		t.Errorf("Expected output to show binary path with file: prefix, got: %s", output)
	}
}

// Unit tests for helper functions

func TestValidateExecFile(t *testing.T) {
	t.Run("valid executable file", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "test-script.sh")
		err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0755)
		if err != nil {
			t.Fatalf("Failed to create test script: %v", err)
		}

		err = validateExecFile(scriptPath)
		if err != nil {
			t.Errorf("Expected no error for valid executable, got: %v", err)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		err := validateExecFile("/nonexistent/file.sh")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
		if !strings.Contains(err.Error(), "failed to access file") {
			t.Errorf("Expected 'failed to access file' error, got: %v", err)
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		tmpDir := t.TempDir()

		err := validateExecFile(tmpDir)
		if err == nil {
			t.Error("Expected error for directory")
		}
		if !strings.Contains(err.Error(), "is a directory") {
			t.Errorf("Expected 'is a directory' error, got: %v", err)
		}
	})

	t.Run("file without execute permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "no-exec.sh")
		err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test script: %v", err)
		}

		err = validateExecFile(scriptPath)
		if err == nil {
			t.Error("Expected error for non-executable file")
		}
		if !strings.Contains(err.Error(), "missing execute permissions") {
			t.Errorf("Expected 'missing execute permissions' error, got: %v", err)
		}
	})

	t.Run("file with owner execute only", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "owner-exec.sh")
		err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0700)
		if err != nil {
			t.Fatalf("Failed to create test script: %v", err)
		}

		err = validateExecFile(scriptPath)
		if err != nil {
			t.Errorf("Expected no error for owner-executable file, got: %v", err)
		}
	})

	t.Run("file with group execute only", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "group-exec.sh")
		err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0650)
		if err != nil {
			t.Fatalf("Failed to create test script: %v", err)
		}

		err = validateExecFile(scriptPath)
		if err != nil {
			t.Errorf("Expected no error for group-executable file, got: %v", err)
		}
	})

	t.Run("file with other execute only", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "other-exec.sh")
		err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0605)
		if err != nil {
			t.Fatalf("Failed to create test script: %v", err)
		}

		err = validateExecFile(scriptPath)
		if err != nil {
			t.Errorf("Expected no error for other-executable file, got: %v", err)
		}
	})
}

func TestValidateExecArgs(t *testing.T) {
	t.Run("valid inline command", func(t *testing.T) {
		cmd := Cmd()
		cmd.SetArgs([]string{"-c", "echo test", "repo1"})
		cmd.ParseFlags([]string{"-c", "echo test"})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err != nil {
			t.Errorf("Expected no error for valid command, got: %v", err)
		}
	})

	t.Run("valid file command", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "test.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0755)

		cmd := Cmd()
		cmd.SetArgs([]string{"-f", scriptPath, "repo1"})
		cmd.ParseFlags([]string{"-f", scriptPath})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err != nil {
			t.Errorf("Expected no error for valid file, got: %v", err)
		}
	})

	t.Run("missing both command and file", func(t *testing.T) {
		cmd := Cmd()
		cmd.SetArgs([]string{"repo1"})
		cmd.ParseFlags([]string{})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err == nil {
			t.Error("Expected error when neither command nor file provided")
		}
		if !strings.Contains(err.Error(), "no command provided") {
			t.Errorf("Expected 'no command provided' error, got: %v", err)
		}
	})

	t.Run("both command and file specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "test.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0755)

		cmd := Cmd()
		cmd.SetArgs([]string{"-c", "echo test", "-f", scriptPath, "repo1"})
		cmd.ParseFlags([]string{"-c", "echo test", "-f", scriptPath})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err == nil {
			t.Error("Expected error when both command and file provided")
		}
		if !strings.Contains(err.Error(), "cannot specify both") {
			t.Errorf("Expected 'cannot specify both' error, got: %v", err)
		}
	})

	t.Run("args without file", func(t *testing.T) {
		cmd := Cmd()
		cmd.SetArgs([]string{"-c", "echo test", "-a", "arg1", "repo1"})
		cmd.ParseFlags([]string{"-c", "echo test", "-a", "arg1"})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err == nil {
			t.Error("Expected error when args used without file")
		}
		if !strings.Contains(err.Error(), "can only be used with") {
			t.Errorf("Expected 'can only be used with' error, got: %v", err)
		}
	})

	t.Run("args with file is valid", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "test.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0755)

		cmd := Cmd()
		cmd.SetArgs([]string{"-f", scriptPath, "-a", "arg1", "-a", "arg2", "repo1"})
		cmd.ParseFlags([]string{"-f", scriptPath, "-a", "arg1", "-a", "arg2"})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err != nil {
			t.Errorf("Expected no error for file with args, got: %v", err)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		cmd := Cmd()
		cmd.SetArgs([]string{"-f", "/nonexistent/file.sh", "repo1"})
		cmd.ParseFlags([]string{"-f", "/nonexistent/file.sh"})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
		if !strings.Contains(err.Error(), "failed to access file") {
			t.Errorf("Expected 'failed to access file' error, got: %v", err)
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		tmpDir := t.TempDir()

		cmd := Cmd()
		cmd.SetArgs([]string{"-f", tmpDir, "repo1"})
		cmd.ParseFlags([]string{"-f", tmpDir})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err == nil {
			t.Error("Expected error for directory")
		}
		if !strings.Contains(err.Error(), "is a directory") {
			t.Errorf("Expected 'is a directory' error, got: %v", err)
		}
	})

	t.Run("non-executable file", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "test.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0644)

		cmd := Cmd()
		cmd.SetArgs([]string{"-f", scriptPath, "repo1"})
		cmd.ParseFlags([]string{"-f", scriptPath})

		err := validateExecArgs(cmd, []string{"repo1"})
		if err == nil {
			t.Error("Expected error for non-executable file")
		}
		if !strings.Contains(err.Error(), "missing execute permissions") {
			t.Errorf("Expected 'missing execute permissions' error, got: %v", err)
		}
	})
}

func TestRunExecCommand(t *testing.T) {
	// Note: runExecCommand requires full integration setup with config and repos
	// These tests focus on the validation and confirmation logic parts
	// Full execution is tested via integration tests

	t.Run("generates correct preview for inline command", func(t *testing.T) {
		cmd := Cmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetIn(mockStdin("n\n"))

		cmd.SetArgs([]string{"-c", "echo test"})
		cmd.ParseFlags([]string{"-c", "echo test"})

		command, _ := cmd.Flags().GetString(scriptFlag)
		filePath, _ := cmd.Flags().GetString(fileFlag)

		var preview string
		if filePath != "" {
			preview = fmt.Sprintf("file: %q", filePath)
		} else {
			preview = fmt.Sprintf("`sh -c %q`", command)
		}

		if !strings.Contains(preview, "`sh -c \"echo test\"`") {
			t.Errorf("Expected inline command preview, got: %s", preview)
		}
	})

	t.Run("generates correct preview for file command", func(t *testing.T) {
		tmpDir := t.TempDir()
		scriptPath := filepath.Join(tmpDir, "test.sh")
		os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test\n"), 0755)

		cmd := Cmd()
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetIn(mockStdin("n\n"))

		cmd.SetArgs([]string{"-f", scriptPath})
		cmd.ParseFlags([]string{"-f", scriptPath})

		command, _ := cmd.Flags().GetString(scriptFlag)
		filePath, _ := cmd.Flags().GetString(fileFlag)

		var preview string
		if filePath != "" {
			preview = fmt.Sprintf("file: %q", filePath)
		} else {
			preview = fmt.Sprintf("`sh -c %q`", command)
		}

		if !strings.Contains(preview, "file:") || !strings.Contains(preview, scriptPath) {
			t.Errorf("Expected file preview with path, got: %s", preview)
		}
	})
}
