package github

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v74/github"
	"github.com/spf13/viper"
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

func New(project string) scm.Provider {
	return &Github{
		// TODO: Add support for enterprise GitHub instances (currently SaaS only)
		client:  github.NewClient(http.DefaultClient).WithAuthToken(viper.GetString(config.AuthToken)),
		project: project,
	}
}

type Github struct {
	client  *github.Client
	project string
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

func readLock() (done func()) {
	sem.Acquire(context.Background(), readWeight)

	return func() {
		sem.Release(readWeight)
	}
}

func writeLock() (done func()) {
	sem.Acquire(context.Background(), writeWeight)

	return func() {
		// delay release to avoid rate limiting
		time.Sleep(viper.GetDuration(config.WriteBackoff))
		sem.Release(writeWeight)
	}
}
