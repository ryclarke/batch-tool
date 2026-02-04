// Package exec provides the shell command execution functionality for batch-tool.
package exec

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
)

const (
	forceFlag  = "force"
	scriptFlag = "script"
	fileFlag   = "file"
	argsFlag   = "arg"
)

// Cmd configures the exec command
func Cmd() *cobra.Command {
	execCmd := &cobra.Command{
		Use:     "exec {-c <command> | -f <file> [-a <arg>]...} [-y] <repository>...",
		Aliases: []string{"sh"},
		Short:   "[!DANGEROUS!] Execute a shell command or file across repositories",
		Long: `Execute a shell command or file across multiple repositories.

This command executes arbitrary shell commands or executable files in the context
of one or more repositories. It can run inline commands via shell evaluation or
execute script files and compiled binaries directly.

WARNING: This command is DANGEROUS. It executes arbitrary code across multiple
repositories without sandboxing. Always review commands carefully before execution.

Command Modes:
  Inline Command (-c):
    Execute a shell command string via 'sh -c'. The command is evaluated in the
    repository's working directory. Use for simple one-liners.

  File Execution (-f):
    Execute a script file or compiled binary directly. The file must have execute
    permissions (chmod +x). Supports shell scripts, Python scripts, binaries, etc.
    Arguments can be passed to the file using one or more -a flags.

Confirmation:
  By default, the command prompts for confirmation before execution, showing the
  command or file that will be executed. Use -y to skip confirmation.`,
		Example: `  # Execute an inline command
  batch-tool exec -c "pwd" repo1 repo2

  # Execute a file
  batch-tool exec -f /path/to/exec repo1 repo2

  # Execute a script with arguments
  batch-tool exec -f ./deploy.sh -a prod -a us-east-1 repo1 repo2`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		PreRunE:           validateExecArgs,
		RunE:              runExecCommand,
	}

	execCmd.Flags().StringP(scriptFlag, "c", "", "shell command to execute")
	execCmd.Flags().StringP(fileFlag, "f", "", "path to an executable file to run")
	execCmd.Flags().StringSliceP(argsFlag, "a", nil, "argument(s) to pass with the command (repeatable, requires -f|--file)")
	execCmd.Flags().BoolP(forceFlag, "y", false, "execute command without asking for confirmation")

	return execCmd
}

// runExecCommand runs the exec command logic based on provided flags
func runExecCommand(cmd *cobra.Command, args []string) error {
	command, filePath, fileArgs, err := getExecArgs(cmd)
	if err != nil {
		return err
	}

	if ok, err := cmd.Flags().GetBool(forceFlag); err != nil {
		return err
	} else if !ok {
		var preview string

		if filePath != "" {
			preview = fmt.Sprintf("file: %q", filePath)
			if len(fileArgs) > 0 {
				preview += fmt.Sprintf(", args: %v", fileArgs)
			}
		} else {
			preview = fmt.Sprintf("`sh -c %q`", command)
		}

		// DOUBLE CHECK with the user before running anything!
		confirmed, err := confirmExecution(cmd.InOrStdin(), cmd.ErrOrStderr(), preview)
		if err != nil {
			return err
		}

		if !confirmed {
			fmt.Fprintln(cmd.ErrOrStderr(), "Aborting.")
			return nil
		}
	}

	// Execute the command or file
	if filePath != "" {
		// Execute the file directly (supports both scripts and binaries)
		call.Do(cmd, args, call.Exec(filePath, fileArgs...))
	} else {
		// Execute inline command via shell evaluation
		call.Do(cmd, args, call.Exec("sh", "-c", command))
	}

	return nil
}

// confirmExecution prompts the user for confirmation and returns true if confirmed
func confirmExecution(in io.Reader, out io.Writer, preview string) (bool, error) {
	fmt.Fprintf(out, "Executing %s\n", preview)
	fmt.Fprintf(out, "Are you sure? [y/N]: ")

	reader := bufio.NewReader(in)
	for {
		confirm, err := reader.ReadString('\n')
		if err != nil {
			return false, err
		}

		switch strings.TrimSpace(strings.ToLower(confirm)) {
		case "no", "n", "":
			// User said no (or provided no response)
			return false, nil

		case "yes", "y":
			// User said yes, proceed with execution
			return true, nil

		default:
			// The response was invalid, so we ask again
			fmt.Fprintf(out, "Expected 'yes' ('y') or 'no' ('n') [y/N]: ")
		}
	}
}

func validateExecArgs(cmd *cobra.Command, _ []string) error {
	command, filePath, fileArgs, err := getExecArgs(cmd)
	if err != nil {
		return err
	}

	// Script args can only be used with file flag
	if len(fileArgs) > 0 && filePath == "" {
		return fmt.Errorf("--%s|-a flags can only be used with --%s|-f", argsFlag, fileFlag)
	}

	// Check that exactly one of command or file is provided
	if command == "" && filePath == "" {
		return fmt.Errorf("no command provided; use the --%s|-c flag to specify a command or --%s|-f to specify a file", scriptFlag, fileFlag)
	}

	if command != "" && filePath != "" {
		return fmt.Errorf("cannot specify both --%s and --%s flags", scriptFlag, fileFlag)
	}

	// If file is provided, verify it is valid for execution
	if filePath != "" {
		if err := validateExecFile(filePath); err != nil {
			return err
		}
	}

	return nil
}

// validateExecFile checks that the provided file exists and is executable
func validateExecFile(filePath string) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to access file %q: %w", filePath, err)
	}

	if fi.IsDir() {
		return fmt.Errorf("provided file path %q is a directory", filePath)
	}

	if fi.Mode().Perm()&0111 == 0 {
		return fmt.Errorf("provided file %q is missing execute permissions", filePath)
	}

	return nil
}

func getExecArgs(cmd *cobra.Command) (command string, filePath string, fileArgs []string, err error) {
	command, err = cmd.Flags().GetString(scriptFlag)
	if err != nil {
		return
	}

	filePath, err = cmd.Flags().GetString(fileFlag)
	if err != nil {
		return
	}

	fileArgs, err = cmd.Flags().GetStringSlice(argsFlag)
	if err != nil {
		return
	}

	return command, filePath, fileArgs, nil
}
