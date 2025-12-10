package exec

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
	"github.com/ryclarke/batch-tool/catalog"
)

// Cmd configures the exec command
func Cmd() *cobra.Command {
	execCmd := &cobra.Command{
		Use:               "exec <repository>...",
		Aliases:           []string{"sh"},
		Short:             "[!DANGEROUS!] Execute a shell command across repositories",
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: catalog.CompletionFunc(),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Import command script from the CLI flag
			command, err := cmd.Flags().GetString("script")
			if err != nil {
				return err
			}

			if ok, err := cmd.Flags().GetBool("force"); err != nil {
				return err
			} else if !ok {
				// DOUBLE CHECK with the user before running anything!
				confirmed, err := confirmExecution(cmd.InOrStdin(), cmd.OutOrStdout(), command, args)
				if err != nil {
					return err
				}

				if !confirmed {
					fmt.Fprintln(cmd.OutOrStdout(), "Aborting.")
					return nil
				}
			}

			call.Do(cmd, args, call.Exec("sh", "-c", command))

			return nil
		},
	}

	execCmd.Flags().StringP("script", "c", "", "shell command(s) to execute")
	execCmd.Flags().BoolP("force", "f", false, "execute command without asking for confirmation")

	return execCmd
}

// confirmExecution prompts the user for confirmation and returns true if confirmed
func confirmExecution(in io.Reader, out io.Writer, exec string, args []string) (bool, error) {
	fmt.Fprintf(out, "Executing command: %v\n", args)
	fmt.Fprintf(out, "  sh -c \"%s\"\n", exec)
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
