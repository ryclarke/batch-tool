package output

import (
	"context"
	"sync"

	"github.com/ryclarke/batch-tool/config"
	"golang.org/x/sync/semaphore"
)

type Channel interface {
	// Name returns the name of the channel.
	Name() string

	// Out returns the output channel for reading.
	Out() <-chan string
	// Err returns the error channel for reading.
	Err() <-chan error

	// WOut returns the output channel for writing.
	WOut() chan<- string
	// WErr returns the error channel for writing.
	WErr() chan<- error

	// Start begins processing with the specified weight for semaphore acquisition.
	// The returned function should be called when processing is complete to release
	// the semaphore and clean up channels. If a waitgroup is provided, it will be
	// decremented when the returned function is called.
	Start(weight int64) (done func(), err error)
}

func NewChannel(ctx context.Context, name string, sem *semaphore.Weighted, wg *sync.WaitGroup) Channel {
	return &channel{
		name:   name,
		output: make(chan string, config.Viper(ctx).GetInt(config.ChannelBuffer)),
		err:    make(chan error, 1),

		ctx: ctx,
		sem: sem,
		wg:  wg,
	}
}

type channel struct {
	name   string
	output chan string
	err    chan error

	ctx context.Context
	sem *semaphore.Weighted
	wg  *sync.WaitGroup

	release bool
}

func (c *channel) Name() string {
	return c.name
}

func (c *channel) Out() <-chan string {
	return c.output
}

func (c *channel) Err() <-chan error {
	return c.err
}

func (c *channel) WOut() chan<- string {
	return c.output
}

func (c *channel) WErr() chan<- error {
	return c.err
}

func (c *channel) Start(weight int64) (func(), error) {
	if weight <= 0 {
		weight = 1 // valid default weight
	}

	done := func() {
		// close channels and signal worker completion
		close(c.output)
		close(c.err)

		// Release semaphore if it was previously acquired
		if c.sem != nil && c.release {
			c.sem.Release(weight)
		}

		// Decrement waitgroup if provided
		if c.wg != nil {
			c.wg.Done()
		}
	}

	if c.sem == nil {
		// No semaphore to acquire, return done function directly
		return done, nil
	}

	if err := c.sem.Acquire(c.ctx, weight); err != nil {
		// Context cancelled, return the error and abort further processing
		return done, err
	}

	c.release = true

	// Acquire semaphore
	return done, nil
}
