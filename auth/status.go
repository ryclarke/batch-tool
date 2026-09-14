package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// fingerprintLength is the number of hex characters of the credential digest
// included in a Status. It is long enough to tell two credentials apart and far
// too short to be useful against a high-entropy secret.
const fingerprintLength = 8

// Status describes the outcome of credential resolution for a single owner. It
// deliberately carries no credential: the digest is one-way and truncated, so a
// Status can be rendered without any risk of exposing the underlying secret.
type Status struct {
	// Owner is the project or organization this status describes.
	Owner string
	// Provider is the backend which resolved (or failed to resolve) the credential.
	Provider string
	// Source describes where the credential came from, or would come from.
	Source string
	// Fingerprint is a truncated SHA-256 digest of the credential. It is empty when
	// no credential was resolved or the owner needs none.
	Fingerprint string
	// Unauthenticated reports whether the owner is deliberately configured without
	// a credential via the "none" backend.
	Unauthenticated bool
	// Err records why resolution failed, if it did.
	Err error
}

// OK reports whether a credential was resolved, or is deliberately not required.
func (s Status) OK() bool {
	return s.Err == nil
}

// Describe resolves the credential for the given host and owner and reports the
// outcome without exposing the credential itself.
func Describe(ctx context.Context, host, owner string) Status {
	scope := ResolveScope(ctx, owner)

	status := Status{
		Owner:    owner,
		Provider: scope.Provider,
		Source:   scope.Source(),
	}

	cred, err := lookup(ctx, host, owner)
	if err != nil {
		status.Err = err

		return status
	}

	// The backend reports its own source, which is more precise than the scope for
	// the auto chain since only resolution reveals which link succeeded.
	if cred.source != "" {
		status.Source = cred.source
	}

	if cred.token == "" {
		status.Unauthenticated = true

		return status
	}

	status.Fingerprint = fingerprint(cred.token)

	return status
}

// DescribeAll reports the credential status for every given owner, preserving the
// order in which they were supplied.
func DescribeAll(ctx context.Context, host string, owners ...string) []Status {
	statuses := make([]Status, 0, len(owners))

	for _, owner := range owners {
		statuses = append(statuses, Describe(ctx, host, owner))
	}

	return statuses
}

// Source describes where this scope's credential comes from, without resolving it.
func (s Scope) Source() string {
	switch s.Provider {
	case BackendEnv:
		return envSource(s.envName())
	case BackendCommand:
		return commandSource(s.Command)
	case BackendGh:
		return ghSource(s.Account)
	case BackendNone:
		return "unauthenticated"
	default:
		return "-"
	}
}

// fingerprint returns a truncated, one-way digest of a credential so that two
// owners can be compared without either credential being displayed.
func fingerprint(token string) string {
	if token == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(token))

	return "sha256:" + hex.EncodeToString(sum[:])[:fingerprintLength]
}

// Sorted returns the given owners deduplicated and sorted, dropping empty names.
func Sorted(owners ...string) []string {
	seen := make(map[string]struct{}, len(owners))
	result := make([]string, 0, len(owners))

	for _, owner := range owners {
		owner = strings.TrimSpace(owner)
		if owner == "" {
			continue
		}

		key := strings.ToLower(owner)
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		result = append(result, owner)
	}

	sort.Strings(result)

	return result
}
