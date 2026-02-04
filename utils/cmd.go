package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ryclarke/batch-tool/config"
)

// Cmd creates an exec.Cmd configured for the given repository context,
// to facilitate consistent environment and working directory setup.
func Cmd(ctx context.Context, repo, command string, arguments ...string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, command, arguments...)
	cmd.Dir = RepoPath(ctx, repo)

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
	branch, _ := LookupBranch(ctx, repo)

	// Start with the inherited environment and add repo-specific metadata
	env := os.Environ()
	env = append(env, fmt.Sprintf("REPO_NAME=%s", repo))
	env = append(env, fmt.Sprintf("GIT_BRANCH=%s", branch))
	env = append(env, fmt.Sprintf("GIT_DEFAULT_BRANCH=%s", CatalogBranchLookup(ctx, repo)))
	env = append(env, fmt.Sprintf("GIT_PROJECT=%s", CatalogProjectLookup(ctx, repo)))

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
