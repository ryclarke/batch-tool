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

		The `Exec` CallFunc builder should be sufficient for most commands, but
		custom CallFunc instances can be defined for more complex scenarios. It
		is also possible to define entire `Wrapper` instances if a specific use
		case requires special handling distinct from the default behavior.
*/
package call

import (
	"bufio"
	"context"
	"os/exec"

	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

// CallFunc defines an atomic unit of work on a repository. Output should
// be sent to the channel, which must remain open. Closing a channel from
// within the context of a CallFunc will result in a panic.
type CallFunc func(ctx context.Context, ch output.Channel) error

// Wrap each provided CallFunc into a new one that executes them order before terminating.
func Wrap(calls ...CallFunc) CallFunc {
	return func(ctx context.Context, ch output.Channel) error {
		// execute each CallFunc, stopping if an error is encountered
		for _, call := range calls {
			if err := call(ctx, ch); err != nil {
				return err
			}
		}

		return nil
	}
}

// Exec creates a new CallFunc to execute the given command and arguments,
// streaming Stdout and Stderr to the channel and returning error status.
func Exec(command string, arguments ...string) CallFunc {
	return func(ctx context.Context, ch output.Channel) error {
		cmd := exec.CommandContext(ctx, command, arguments...)
		cmd.Dir = utils.RepoPath(ctx, ch.Name())
		cmd.Env = utils.ExecEnv(ctx, ch.Name())

		// Configure the pipe for stdout
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}

		// Merge stderr to the stdout pipe
		cmd.Stderr = cmd.Stdout

		if err := cmd.Start(); err != nil {
			return err
		}

		// stream output to the channel as it becomes available
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			ch.WriteString(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			return err
		}

		return cmd.Wait()
	}
}
