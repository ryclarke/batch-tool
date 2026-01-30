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
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ryclarke/batch-tool/catalog"
	"github.com/ryclarke/batch-tool/config"
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
		cmd, err := Cmd(ctx, ch.Name(), command, arguments...)
		if err != nil {
			return err
		}

		// Directly use channel as io.Writer
		cmd.Stdout, cmd.Stderr = ch, ch

		return cmd.Run()
	}
}

// Cmd creates an exec.Cmd configured for the given repository context,
// to facilitate consistent environment and working directory setup.
func Cmd(ctx context.Context, repo, command string, arguments ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, arguments...)
	cmd.Dir = utils.RepoPath(ctx, repo)

	env, err := Env(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to construct environment for %q: %w", repo, err)
	}

	cmd.Env = env

	return cmd, nil
}

// Env constructs the environment variables for an Exec call.
// It processes the CmdEnv config which can contain either:
// - key=value pairs (used as-is)
// - file paths to .env files (read and all values added to environment)
// If a file path is provided but cannot be read, an error is returned.
func Env(ctx context.Context, repo string) ([]string, error) {
	viper := config.Viper(ctx)
	branch, _ := utils.LookupBranch(ctx, repo)

	// Start with the inherited environment and add repo-specific metadata
	env := os.Environ()
	env = append(env, fmt.Sprintf("REPO_NAME=%s", repo))
	env = append(env, fmt.Sprintf("GIT_BRANCH=%s", branch))
	env = append(env, fmt.Sprintf("GIT_DEFAULT_BRANCH=%s", catalog.GetBranchForRepo(ctx, repo)))
	env = append(env, fmt.Sprintf("GIT_PROJECT=%s", catalog.GetProjectForRepo(ctx, repo)))

	// Add user-specified environment variables
	envArgs := viper.GetStringSlice(config.CmdEnv)
	for _, envArg := range envArgs {
		if strings.Contains(envArg, "=") {
			// It's a key=value pair, use as-is
			env = append(env, envArg)
		} else {
			// Treat as file path and try to read it as an envfile
			fileEnvs, err := parseEnvFile(envArg)
			if err != nil {
				return nil, err
			}

			env = append(env, fileEnvs...)
		}
	}

	return env, nil
}

func parseEnvFile(envArg string) ([]string, error) {
	envs := make([]string, 0)

	content, err := os.ReadFile(envArg)
	if err != nil {
		return nil, fmt.Errorf("failed to read envfile %q: %w", envArg, err)
	}

	// Parse the envfile and add each line as an environment variable
	for line := range strings.SplitSeq(string(content), "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.Contains(line, "=") {
			envs = append(envs, line)
		}
	}

	return envs, nil
}
