package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v74/github"
	"golang.org/x/sync/semaphore"

	"github.com/ryclarke/batch-tool/config"
	"github.com/ryclarke/batch-tool/scm"
)

const (
	// weights designed to avoid secondary rate limiting for creative requests
	// https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
	writeWeight = 10
	readWeight  = 1
)

var sem = semaphore.NewWeighted(writeWeight)

var _ scm.Provider = new(Github)

func init() {
	// Register the GitHub provider factory
	scm.Register("github", New)
}

// New creates a new GitHub provider instance.
func New(ctx context.Context, project string) scm.Provider {
	viper := config.Viper(ctx)
	return &Github{
		// TODO: Add support for enterprise GitHub instances (currently SaaS only)
		client:  github.NewClient(http.DefaultClient).WithAuthToken(viper.GetString(config.AuthToken)),
		project: project,
		ctx:     ctx,
	}
}

// Github implements the scm.Provider interface for GitHub.
type Github struct {
	client  *github.Client
	project string
	ctx     context.Context
}

// handleRateLimitError checks if the error is a rate limit error and waits for the limit to reset.
// Returns true if a retry should be attempted, false if the error is not rate-limit related.
// The search parameter indicates whether to check search rate limits (true) or core rate limits (false).
func (g *Github) handleRateLimitError(ctx context.Context, err error, search bool) (shouldRetry bool, retErr error) {
	rateLimitError := &github.RateLimitError{}
	if !errors.As(err, &rateLimitError) {
		// Not a rate limit error, don't retry
		return false, nil
	}

	// It's a rate limit error, wait for reset
	if rateErr := g.waitForRateLimit(ctx, search); rateErr != nil {
		return false, rateErr
	}

	return true, nil
}

func (g *Github) waitForRateLimit(ctx context.Context, search bool) error {
	rate, err := g.checkRateLimit(ctx, search)
	if err != nil {
		return err
	}

	// inform the user of the wait time to expect
	fmt.Fprintf(os.Stderr, "... rate limit exceeded, waiting until %s ...\n", rate.Reset.GetTime().Format(time.RFC1123))

	// wait until rate limit resets, plus a buffer
	time.Sleep(time.Until(*rate.Reset.GetTime()) + 2*time.Second)

	return nil
}

func (g *Github) checkRateLimit(ctx context.Context, search bool) (*github.Rate, error) {
	limits, _, err := g.client.RateLimit.Get(ctx)
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
		// Context cancelled or deadline exceeded, return no-op cleanup
		return func() {}
	}

	return func() {
		sem.Release(readWeight)
	}
}

func (g *Github) writeLock() (done func()) {
	if err := sem.Acquire(g.ctx, writeWeight); err != nil {
		// Context cancelled or deadline exceeded, return no-op cleanup
		return func() {}
	}

	return func() {
		// delay release to avoid rate limiting
		time.Sleep(config.Viper(g.ctx).GetDuration(config.WriteBackoff))
		sem.Release(writeWeight)
	}
}
