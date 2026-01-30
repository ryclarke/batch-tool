// Package output provides output handling and formatting for batch operations.
package output

import (
	"context"
	"io"
	"sync"

	"golang.org/x/sync/semaphore"

	"github.com/ryclarke/batch-tool/config"
)

// Channel represents an output channel for concurrent operations
type Channel interface {
	// Name returns the name of the channel.
	Name() string

	// Out returns the output channel for reading.
	Out() <-chan []byte
	// Err returns the error channel for reading.
	Err() <-chan error

	// WriteString writes a whole line to the output channel as a string.
	io.StringWriter
	// Write bytes directly to the output channel.
	io.Writer

	// WriteError writes an error to the error channel.
	WriteError(err error)

	// Start begins processing with the specified weight for semaphore acquisition.
	Start(weight int64) error
	// Close the channels and release any acquired semaphore. If a wait group is
	// provided it will be decremented.
	io.Closer
}

// NewChannel creates a new output channel with the given context, name, semaphore, and wait group
func NewChannel(ctx context.Context, name string, sem *semaphore.Weighted, wg *sync.WaitGroup) Channel {
	return &channel{
		name:   name,
		output: make(chan []byte, config.Viper(ctx).GetInt(config.ChannelBuffer)),
		err:    make(chan error, 1),

		ctx: ctx,
		sem: sem,
		wg:  wg,
	}
}

type channel struct {
	name   string
	output chan []byte
	err    chan error

	ctx context.Context
	sem *semaphore.Weighted
	wg  *sync.WaitGroup

	mu      sync.Mutex // protects concurrent writes from Stdout/Stderr
	release int64      // weight to release (for semaphore)
}

func (c *channel) Name() string {
	return c.name
}

func (c *channel) Out() <-chan []byte {
	return c.output
}

func (c *channel) Err() <-chan error {
	return c.err
}

func (c *channel) Write(p []byte) (n int, _ error) {
	if len(p) == 0 {
		return 0, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Make a copy to prevent caller from modifying the buffer
	buf := make([]byte, len(p))
	copy(buf, p)
	c.output <- buf

	return len(p), nil
}

// WriteString writes a string to the output channel and always returns a nil error.
// Each string is treated as a line and terminated with a newline in the output.
func (c *channel) WriteString(s string) (n int, _ error) {
	if len(s) == 0 {
		return 0, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.output <- []byte(s + "\n")
	return len(s), nil
}

// WriteError writes an error to the error channel.
func (c *channel) WriteError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

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
