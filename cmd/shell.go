package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ryclarke/batch-tool/call"
)

func addShellCmd() *cobra.Command {
	// shellCmd represents the shell command (hidden)
	shellCmd := &cobra.Command{
		Use:     "shell <repository> ...",
		Aliases: []string{"sh"},
		Short:   "[!DANGEROUS!] Execute a shell command across repositories",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Import command(s) from the CLI flag
			exec, err := cmd.Flags().GetString("exec")
			if err != nil {
				return err
			}

			// DOUBLE CHECK with the user before running anything!
			fmt.Printf("Executing command: %v\n", args)
			fmt.Printf("  sh -c \"%s\"\n", exec)
			fmt.Printf("Are you sure? [y/N]: ")

			var done bool

			for !done {
				confirm, err := bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return err
				}

				switch strings.TrimSpace(strings.ToLower(confirm)) {
				case "no", "n", "":
					// User said no (or provided no response), exit without doing anything
					fmt.Println("Aborting.")
					return nil

				case "yes", "y":
					// User said yes, proceed with execution
					done = true
				default:
					// The response was invalid, so we ask again
					fmt.Printf("Expected 'yes' ('y') or 'no' ('n') [y/N]: ")
				}
			}

			call.Do(args, cmd.OutOrStdout(), call.Wrap(call.Exec("sh", "-c", exec)))

			return nil
		},
	}

	shellCmd.Flags().StringP("exec", "c", "", "shell command(s) to execute")

	return shellCmd
}
