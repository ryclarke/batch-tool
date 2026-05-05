package call

import (
	"context"

	"github.com/ryclarke/batch-tool/output"
	"github.com/ryclarke/batch-tool/utils"
)

// Func defines an atomic unit of work on a repository. Output should
// be sent to the channel, which must remain open. Closing a channel from
// within the context of a Func will result in a panic.
type Func func(ctx context.Context, ch output.Channel) error

// Wrap each provided Func into a new one that executes them in order before terminating.
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
		cmd, err := utils.Cmd(ctx, ch.Name(), command, arguments...)
		if err != nil {
			return err
		}

		// Directly use channel as io.Writer
		cmd.Stdout, cmd.Stderr = ch, ch

		return cmd.Run()
	}
}

// Error wraps runtime errors that occur during subprocess execution.
type Error struct {
	error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.error != nil {
		return e.error.Error()
	}
	return ""
}

// Unwrap returns the wrapped error for error chain inspection.
func (e *Error) Unwrap() error {
	return e.error
}

// Is allows errors.Is to identify this as a call.Error.
func (e *Error) Is(target error) bool {
	_, ok := target.(*Error)
	return ok
}
