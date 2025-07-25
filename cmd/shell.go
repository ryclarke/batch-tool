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
		Hidden:  true,
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
			fmt.Printf("Are you sure? ")

			var confirm string

			for confirm != "yes" && confirm != "no" {
				fmt.Printf("[yes/no] ")

				confirm, err = bufio.NewReader(os.Stdin).ReadString('\n')
				if err != nil {
					return err
				}

				// strip the trailing newline and make lower
				confirm = strings.TrimSpace(strings.ToLower(confirm))
			}

			if confirm == "no" {
				return nil
			}

			call.Do(args, call.Wrap(call.Exec("sh", "-c", exec)))

			return nil
		},
	}

	shellCmd.Flags().StringP("exec", "c", "", "shell command(s) to execute")

	return shellCmd
}
