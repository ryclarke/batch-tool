package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v74/github"
	"golang.org/x/sync/semaphore"

	"github.com/ryclarke/batch-tool/auth"
	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

const githubSaaSHost = "github.com"

const (
	// weights designed to avoid secondary rate limiting for creative requests
	// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
	writeWeight = 10
	readWeight  = 1
)

var (
	sem  = semaphore.NewWeighted(writeWeight)
	caps = &scm.Capabilities{
		TeamReviewers:  true,
		ResetReviewers: true,
		Draft:          true,

		MergeMethods:   []string{"merge", "squash", "rebase"},
		CheckMergeable: true,
	}
)

var _ scm.Provider = new(Github)

func init() {
	// Register the GitHub provider factory
	scm.Register("github", New)
}

// New creates a new GitHub provider instance.
func New(ctx context.Context, project string) scm.Provider {
	viper := config.Viper(ctx)
	host := cleanHostname(viper.GetString(config.GitHost))

	// Credentials are resolved per project so that each configured organization can
	// authenticate with a different account.
	token, authErr := auth.Token(ctx, host, project)
	client := github.NewClient(http.DefaultClient).WithAuthToken(token)

	if host != githubSaaSHost && host != "" {
		var err error

		if client, err = client.WithEnterpriseURLs(
			fmt.Sprintf("https://%s/api/v3/", host),
			fmt.Sprintf("https://%s/api/uploads/", host),
		); err != nil {
			panic(fmt.Sprintf("github: invalid enterprise URLs for host %q: %v", host, err))
		}
	}

	return &Github{
		client:  client,
		project: project,
		ctx:     ctx,
		authErr: authErr,
	}
}

// Github implements the scm.Provider interface for GitHub.
type Github struct {
	client  *github.Client
	project string
	ctx     context.Context

	// authErr records a credential resolution failure from New, which cannot
	// return an error through the scm.ProviderFactory signature.
	authErr error
}

// CheckCapabilities validates that the provided PR options are supported by GitHub.
// The check is local to the static capability set and issues no request, so it
// requires no credential.
func (g *Github) CheckCapabilities(opts *scm.PROptions) error {
	return scm.ValidatePROptions(caps, opts)
}

// handleRateLimitError checks if the error is a rate limit error and waits for the limit to reset.
// Returns true if a retry should be attempted, false if the error is not rate-limit related.
// The search parameter indicates whether to check search rate limits (true) or core rate limits (false).
func (g *Github) handleRateLimitError(err error, search bool) (shouldRetry bool, retErr error) {
	rateLimitError := &github.RateLimitError{}
	if !errors.As(err, &rateLimitError) {
		// Not a rate limit error, don't retry
		return false, nil
	}

	// It's a rate limit error, wait for reset
	if rateErr := g.waitForRateLimit(search); rateErr != nil {
		return false, rateErr
	}

	return true, nil
}

func (g *Github) waitForRateLimit(search bool) error {
	rate, err := g.checkRateLimit(search)
	if err != nil {
		return err
	}

	// inform the user of the wait time to expect
	fmt.Fprintf(os.Stderr, "... rate limit exceeded, waiting until %s ...\n", rate.Reset.GetTime().Format(time.RFC1123))

	// wait until rate limit resets, plus a buffer
	time.Sleep(time.Until(*rate.Reset.GetTime()) + 2*time.Second)

	return nil
}

func (g *Github) checkRateLimit(search bool) (*github.Rate, error) {
	limits, _, err := g.client.RateLimit.Get(g.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check rate limits: %w", err)
	}

	if search {
		return limits.Search, nil
	}

	return limits.Core, nil
}

func (g *Github) readLock() (done func()) {
	if err := sem.Acquire(g.ctx, readWeight); err != nil {
		return func() {
			// Context cancelled or deadline exceeded, return no-op cleanup
		}
	}

	return func() {
		sem.Release(readWeight)
	}
}

func (g *Github) writeLock() (done func()) {
	if err := sem.Acquire(g.ctx, writeWeight); err != nil {
		return func() {
			// Context cancelled or deadline exceeded, return no-op cleanup
		}
	}

	return func() {
		// delay release to avoid rate limiting
		time.Sleep(config.Viper(g.ctx).GetDuration(config.WriteBackoff))
		sem.Release(writeWeight)
	}
}

// cleanHostname normalizes the GitHub host by removing protocol prefixes and trailing slashes.
func cleanHostname(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")
	return host
}
