package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cast"
	"golang.org/x/sync/singleflight"

	"github.com/ryclarke/batch-tool/config"
)

// ErrNoCredential is returned when no configured backend yields a credential.
var ErrNoCredential = errors.New("no credential found")

// Supported credential backends, selected via the auth.provider setting.
const (
	// BackendAuto tries each available source in turn. This is the default.
	BackendAuto = "auto"
	// BackendEnv reads the credential from the environment variable named by auth.env.
	BackendEnv = "env"
	// BackendGh reads the credential from the gh CLI (GitHub only).
	BackendGh = "gh"
	// BackendCommand reads the credential from the stdout of an external command.
	BackendCommand = "command"
	// BackendNone makes unauthenticated requests.
	BackendNone = "none"
)

// Backends lists every supported auth.provider value.
var Backends = []string{BackendAuto, BackendEnv, BackendGh, BackendCommand, BackendNone}

// credential pairs a resolved token with a description of where it came from.
// The source is safe to display; the token never is.
type credential struct {
	token  string
	source string
}

// cache holds credentials resolved during this process. It is deliberately
// process-local: credentials are never written to viper (config.Child copies
// AllSettings into every per-repo instance) nor persisted to disk.
var cache = struct {
	sync.RWMutex
	entries map[string]credential

	group singleflight.Group
}{entries: make(map[string]credential)}

// Scope is the set of credential settings which apply to a single owner. Values
// are merged from the top-level auth settings and any matching auth.owners entry.
type Scope struct {
	// Owner is the project or organization these settings apply to.
	Owner string
	// Provider names the backend used to resolve the credential.
	Provider string
	// Env names the environment variable holding the credential. It never holds
	// the credential itself.
	Env string
	// Command is the argv of an external command whose stdout is the credential.
	Command []string
	// Account optionally selects a specific gh CLI account.
	Account string
}

// Token returns the credential for the given host and owner, resolving it through
// the configured backend on first use and caching the result in memory. An empty
// token with a nil error means the owner is configured for unauthenticated access.
func Token(ctx context.Context, host, owner string) (string, error) {
	cred, err := lookup(ctx, host, owner)

	return cred.token, err
}

// Validate reports whether a credential can be resolved for the given host and
// owner, without exposing the credential itself.
func Validate(ctx context.Context, host, owner string) error {
	_, err := lookup(ctx, host, owner)

	return err
}

// lookup resolves and caches the credential for the given host and owner.
func lookup(ctx context.Context, host, owner string) (credential, error) {
	key := cacheKey(host, owner)

	cache.RLock()
	cred, ok := cache.entries[key]
	cache.RUnlock()

	if ok {
		return cred, nil
	}

	// Deduplicate concurrent lookups so a cold cache doesn't spawn one subprocess
	// per repository when call.Do fans out across many goroutines.
	result, err, _ := cache.group.Do(key, func() (any, error) {
		cred, err := resolve(ctx, host, ResolveScope(ctx, owner))
		if err != nil {
			return credential{}, err
		}

		cache.Lock()
		cache.entries[key] = cred
		cache.Unlock()

		return cred, nil
	})
	if err != nil {
		return credential{}, err
	}

	return result.(credential), nil
}

// ResolveScope merges the auth.owners entry for the given owner over the top-level
// auth settings. An owner entry only overrides the fields it sets.
func ResolveScope(ctx context.Context, owner string) Scope {
	viper := config.Viper(ctx)

	scope := Scope{
		Owner:    owner,
		Provider: viper.GetString(config.AuthProvider),
		Env:      viper.GetString(config.AuthEnv),
		Command:  viper.GetStringSlice(config.AuthCommand),
		Account:  viper.GetString(config.AuthAccount),
	}

	if scope.Provider == "" {
		scope.Provider = BackendAuto
	}

	if scope.Env == "" {
		scope.Env = config.DefaultAuthEnv
	}

	// Viper lowercases all configuration keys, and owner names are case-insensitive
	// on every supported provider, so match on the lowercased name.
	entry, ok := ownerEntry(viper.GetStringMap(config.AuthOwners), owner)
	if !ok {
		return scope
	}

	if value, ok := entry["provider"]; ok {
		if provider := strings.TrimSpace(cast.ToString(value)); provider != "" {
			scope.Provider = provider
		}
	}

	if value, ok := entry["env"]; ok {
		if env := strings.TrimSpace(cast.ToString(value)); env != "" {
			scope.Env = env
		}
	}

	if value, ok := entry["command"]; ok {
		if command := cast.ToStringSlice(value); len(command) > 0 {
			scope.Command = command
		}
	}

	if value, ok := entry["account"]; ok {
		if account := strings.TrimSpace(cast.ToString(value)); account != "" {
			scope.Account = account
		}
	}

	return scope
}

// Owners returns the owners which have an explicit auth.owners entry.
func Owners(ctx context.Context) []string {
	owners := make([]string, 0)

	for owner := range config.Viper(ctx).GetStringMap(config.AuthOwners) {
		if owner = strings.TrimSpace(owner); owner != "" {
			owners = append(owners, owner)
		}
	}

	return owners
}

// ownerEntry looks up an owner's settings from the auth.owners map, tolerating
// both nested maps and case differences in the configured key.
func ownerEntry(owners map[string]any, owner string) (map[string]any, bool) {
	owner = strings.ToLower(strings.TrimSpace(owner))
	if owner == "" || len(owners) == 0 {
		return nil, false
	}

	for key, value := range owners {
		if strings.ToLower(strings.TrimSpace(key)) != owner {
			continue
		}

		entry := cast.ToStringMap(value)
		if entry == nil {
			return nil, false
		}

		// Normalize field names so lookups are case-insensitive regardless of
		// whether the values came from a config file or a viper.Set call in tests.
		normalized := make(map[string]any, len(entry))
		for field, fieldValue := range entry {
			normalized[strings.ToLower(strings.TrimSpace(field))] = fieldValue
		}

		return normalized, true
	}

	return nil, false
}

func cacheKey(host, owner string) string {
	return strings.ToLower(strings.TrimSpace(host)) + "|" + strings.ToLower(strings.TrimSpace(owner))
}

// Reset clears the in-memory credential cache. It exists for tests and for
// commands which need to re-resolve credentials after changing configuration.
func Reset() {
	cache.Lock()
	defer cache.Unlock()

	cache.entries = make(map[string]credential)
	cache.group = singleflight.Group{}

	deprecationOnce = sync.Once{}
}

// envSource renders an environment variable name for display.
func envSource(name string) string {
	return "$" + name
}

// commandSource renders a command name for display, omitting its arguments so
// that nothing which might carry sensitive values is echoed.
func commandSource(command []string) string {
	if len(command) == 0 {
		return "(auth.command not set)"
	}

	return command[0]
}

// ghSource renders the gh account selection for display.
func ghSource(account string) string {
	if account != "" {
		return fmt.Sprintf("gh (user: %s)", account)
	}

	return "gh (active account)"
}
