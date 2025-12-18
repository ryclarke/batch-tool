/*
Package call provides helpers for managing and executing asynchronous work

	 across multiple repositories. Commands for this batch tool shall execute
	 `Do(...)` with a Wrapper containing all of the tasks for that command.

	 Example:
	 	repos := []string{"repo1", "repo2", "repo3"}
			fwrap := Wrap(Exec("git", "status"), Exec("ls"))
			Do(repos, fwrap)

		The above example code calls `git status` followed by `ls` on all three
		provided repositories, executing asynchronously and printing the output
		in order. Console output will block iteratively across the repository
		list to ensure that the output isn't mixed, but the processing of each
		repository's respective tasks is fully parallel in the background.

		The `Exec` Func builder should be sufficient for most commands, but
		custom Func instances can be defined for more complex scenarios. It
		is also possible to define entire `Wrapper` instances if a specific use
		case requires special handling distinct from the default behavior.
*/
package call

import (
	"context"
	"os/exec"

	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

// Func defines an atomic unit of work on a repository. Output should
// be sent to the channel, which must remain open. Closing a channel from
// within the context of a Func will result in a panic.
type Func func(ctx context.Context, ch output.Channel) error

// Wrap each provided Func into a new one that executes them order before terminating.
func Wrap(calls ...Func) Func {
	return func(ctx context.Context, ch output.Channel) error {
		// execute each Func, stopping if an error is encountered
		for _, call := range calls {
			if err := call(ctx, ch); err != nil {
				return err
			}
		}

		return nil
	}
}

// Exec creates a new Func to execute the given command and arguments,
// streaming Stdout and Stderr to the channel and returning error status.
func Exec(command string, arguments ...string) Func {
	return func(ctx context.Context, ch output.Channel) error {
		cmd := exec.CommandContext(ctx, command, arguments...)
		cmd.Dir = utils.RepoPath(ctx, ch.Name())
		cmd.Env = utils.ExecEnv(ctx, ch.Name())

		// Directly use channel as io.Writer
		cmd.Stdout, cmd.Stderr = ch, ch

		return cmd.Run()
	}
}
