package auth

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ghCommand is the name of the GitHub CLI executable.
const ghCommand = "gh"

// Indirection points for tests, following the same package-level func var
// convention used by utils.CatalogProjectLookup.
var (
	lookPath = exec.LookPath

	runCmd = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		// The command and its arguments come from configuration by design, so that
		// credentials can be read from an external tool. They are passed as argv and
		// never interpreted by a shell, so configured values cannot inject additional
		// commands.
		return exec.CommandContext(ctx, name, args...).Output() //nolint:gosec // argv-only execution, no shell
	}
)

// ghAvailable reports whether the gh CLI is installed. The CLI is optional:
// the auto backend skips it when absent rather than failing.
func ghAvailable() bool {
	_, err := lookPath(ghCommand)

	return err == nil
}

// ghToken retrieves a credential from the gh CLI. The --user flag disambiguates
// between multiple accounts authenticated against the same host, which is what
// allows separate accounts per project without switching the active gh account.
func ghToken(ctx context.Context, host string, scope Scope) (string, error) {
	args := []string{"auth", "token"}

	if host = strings.TrimSpace(host); host != "" {
		args = append(args, "--hostname", host)
	}

	if scope.Account != "" {
		args = append(args, "--user", scope.Account)
	}

	stdout, err := runCmd(ctx, ghCommand, args...)
	if err != nil {
		return "", fmt.Errorf("gh auth token failed for %s: %w%s", describeOwner(scope), err, formatStderr(err))
	}

	// gh writes the token to stdout, so it must not appear in any error below.
	token := strings.TrimSpace(string(stdout))
	if token == "" {
		return "", fmt.Errorf("%w for %s: gh auth token returned no token (try `gh auth login`)",
			ErrNoCredential, describeOwner(scope))
	}

	return token, nil
}

// formatStderr renders the stderr captured by exec for inclusion in an error.
// Only stderr is ever surfaced - stdout may contain a credential.
func formatStderr(err error) string {
	exitErr := &exec.ExitError{}
	if !errors.As(err, &exitErr) {
		return ""
	}

	stderr := strings.TrimSpace(string(exitErr.Stderr))
	if stderr == "" {
		return ""
	}

	return ": " + stderr
}
