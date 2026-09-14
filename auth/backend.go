package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/ryclarke/batch-tool/config"
)

// githubProvider is the git.provider value which enables GitHub-specific
// credential sources (GITHUB_TOKEN and the gh CLI).
const githubProvider = "github"

// githubEnvVars are the idiomatic GitHub Actions credential variables, tried in
// order so that CI runs authenticate without any additional configuration.
var githubEnvVars = []string{"GH_TOKEN", "GITHUB_TOKEN"}

// deprecationOnce guards the auth-token deprecation notice so it is printed at
// most once per process rather than once per repository.
var deprecationOnce sync.Once

// resolve runs the backend selected by the scope and returns the credential
// together with a description of where it came from.
func resolve(ctx context.Context, host string, scope Scope) (credential, error) {
	switch scope.Provider {
	case BackendNone:
		return credential{source: "unauthenticated"}, nil
	case BackendEnv:
		return fromEnv(scope)
	case BackendCommand:
		return fromCommand(ctx, scope)
	case BackendGh:
		return fromGh(ctx, host, scope)
	case BackendAuto, "":
		return fromAuto(ctx, host, scope)
	default:
		return credential{}, fmt.Errorf("unknown auth provider %q for %s (expected one of %s)",
			scope.Provider, describeOwner(scope), strings.Join(Backends, ", "))
	}
}

// fromEnv reads the credential from the environment variable named by auth.env.
func fromEnv(scope Scope) (credential, error) {
	name := scope.envName()

	if token := strings.TrimSpace(os.Getenv(name)); token != "" {
		return credential{token: token, source: envSource(name)}, nil
	}

	return credential{}, fmt.Errorf("%w for %s: environment variable %s is not set",
		ErrNoCredential, describeOwner(scope), name)
}

// fromCommand reads the credential from the stdout of an external command. The
// command is executed directly from its argv and is never passed through a shell.
func fromCommand(ctx context.Context, scope Scope) (credential, error) {
	if len(scope.Command) == 0 {
		return credential{}, fmt.Errorf("auth provider %q for %s requires auth.command to be set",
			BackendCommand, describeOwner(scope))
	}

	stdout, err := runCmd(ctx, scope.Command[0], scope.Command[1:]...)
	if err != nil {
		// Report the command name and its stderr only - stdout holds the credential.
		return credential{}, fmt.Errorf("auth command %q failed for %s: %w%s",
			scope.Command[0], describeOwner(scope), err, formatStderr(err))
	}

	token := strings.TrimSpace(string(stdout))
	if token == "" {
		return credential{}, fmt.Errorf("%w for %s: auth command %q produced no output",
			ErrNoCredential, describeOwner(scope), scope.Command[0])
	}

	return credential{token: token, source: commandSource(scope.Command)}, nil
}

// fromGh reads the credential from the gh CLI.
func fromGh(ctx context.Context, host string, scope Scope) (credential, error) {
	if !isGithub(ctx) {
		return credential{}, fmt.Errorf("auth provider %q for %s requires git.provider to be %q",
			BackendGh, describeOwner(scope), githubProvider)
	}

	if !ghAvailable() {
		return credential{}, fmt.Errorf("auth provider %q for %s requires the gh CLI: install it from https://cli.github.com or choose another auth provider",
			BackendGh, describeOwner(scope))
	}

	token, err := ghToken(ctx, host, scope)
	if err != nil {
		return credential{}, err
	}

	return credential{token: token, source: ghSource(scope.Account)}, nil
}

// fromAuto tries each available credential source in turn. Sources the user
// configured explicitly are preferred over discovered ones, so that enabling the
// gh CLI never silently replaces a credential which was already configured.
func fromAuto(ctx context.Context, host string, scope Scope) (credential, error) {
	name := scope.envName()

	if token := strings.TrimSpace(os.Getenv(name)); token != "" {
		return credential{token: token, source: envSource(name)}, nil
	}

	// The deprecated single-token setting is an explicit choice, so it takes
	// precedence over the discovered sources below.
	if token := strings.TrimSpace(config.Viper(ctx).GetString(config.AuthToken)); token != "" {
		// The value can only have come from the config file when the default
		// environment variable is unset, so the notice can safely name the key.
		if os.Getenv(config.DefaultAuthEnv) == "" {
			deprecationOnce.Do(func() {
				fmt.Fprintf(os.Stderr, "WARNING: the %q setting is deprecated - set %q to the name of an environment variable instead\n\n",
					config.AuthToken, config.AuthEnv)
			})
		}

		return credential{token: token, source: config.AuthToken + " (deprecated)"}, nil
	}

	if isGithub(ctx) {
		// GH_TOKEN and GITHUB_TOKEN keep CI runs deterministic without spawning a
		// subprocess, so they are tried before the gh CLI.
		for _, env := range githubEnvVars {
			if token := strings.TrimSpace(os.Getenv(env)); token != "" {
				return credential{token: token, source: envSource(env)}, nil
			}
		}

		// The gh CLI is optional, so a failure here falls through rather than
		// aborting resolution.
		if ghAvailable() {
			if token, err := ghToken(ctx, host, scope); err == nil {
				return credential{token: token, source: ghSource(scope.Account)}, nil
			}
		}
	}

	return credential{}, fmt.Errorf("%w for %s: set %s, configure auth.provider, or authenticate with the gh CLI",
		ErrNoCredential, describeOwner(scope), name)
}

// envName returns the environment variable this scope reads its credential from.
func (s Scope) envName() string {
	if s.Env == "" {
		return config.DefaultAuthEnv
	}

	return s.Env
}

func isGithub(ctx context.Context) bool {
	return strings.EqualFold(strings.TrimSpace(config.Viper(ctx).GetString(config.GitProvider)), githubProvider)
}

func describeOwner(scope Scope) string {
	if scope.Owner == "" {
		return "the default project"
	}

	return "project " + scope.Owner
}
