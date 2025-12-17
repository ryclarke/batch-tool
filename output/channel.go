package output

import (
	"context"
	"io"
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

	// WriteString writes a string to the output channel.
	io.StringWriter

	// WriteError writes an error to the error channel.
	WriteError(err error)

	// Start begins processing with the specified weight for semaphore acquisition.
	Start(weight int64) error
	// Close the channels and release any acquired semaphore. If a wait group is
	// provided it will be decremented.
	io.Closer
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

	release int64 // weight to release (for semaphore)
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

// WriteString writes a string to the output channel and always returns a nil error.
func (c *channel) WriteString(s string) (n int, _ error) {
	c.output <- s
	return len(s), nil
}

// WriteError writes an error to the error channel.
func (c *channel) WriteError(err error) {
	c.err <- err
}

func (c *channel) Start(weight int64) error {
	if weight <= 0 {
		weight = 1 // valid default weight
	}

	if c.sem == nil {
		// No semaphore to acquire, return immediately
		return nil
	}

	if err := c.sem.Acquire(c.ctx, weight); err != nil {
		return err
	}

	// Acquired semaphore successfully, set release weight
	c.release = weight

	return nil
}

func (c *channel) Close() error {
	// close channels and signal worker completion
	close(c.output)
	close(c.err)

	// Release semaphore if it was previously acquired
	if c.sem != nil && c.release > 0 {
		c.sem.Release(c.release)
	}

	// Decrement waitgroup if provided
	if c.wg != nil {
		c.wg.Done()
	}

	return nil
}
