package output

import (
	"context"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
)

func TestNewChannel(t *testing.T) {
	ctx := context.Background()
	sem := semaphore.NewWeighted(10)
	var wg sync.WaitGroup

	t.Run("creates channel with all components", func(t *testing.T) {
		ch := NewChannel(ctx, "test-repo", sem, &wg)

		if ch == nil {
			t.Fatal("Expected NewChannel to return non-nil channel")
		}

		if ch.Name() != "test-repo" {
			t.Errorf("Expected name 'test-repo', got %s", ch.Name())
		}

		if ch.Out() == nil {
			t.Error("Expected Out() to return non-nil channel")
		}

		if ch.Err() == nil {
			t.Error("Expected Err() to return non-nil channel")
		}
	})

	t.Run("creates channel without semaphore", func(t *testing.T) {
		ch := NewChannel(ctx, "test-repo", nil, nil)

		if ch == nil {
			t.Fatal("Expected NewChannel to return non-nil channel")
		}

		if ch.Name() != "test-repo" {
			t.Errorf("Expected name 'test-repo', got %s", ch.Name())
		}
	})
}

func Test_channel_Name(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			name: "returns channel name",
			want: "test-repo",
		},
		{
			name: "handles empty name",
			want: "",
		},
		{
			name: "handles special characters",
			want: "org/repo-name_123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &channel{
				name:   tt.want,
				output: make(chan string),
				err:    make(chan error),
				ctx:    context.Background(),
			}
			if got := c.Name(); got != tt.want {
				t.Errorf("channel.Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_channel_Out(t *testing.T) {
	t.Run("returns output channel for reading", func(t *testing.T) {
		outChan := make(chan string)
		c := &channel{
			name:   "test",
			output: outChan,
			err:    make(chan error),
			ctx:    context.Background(),
		}

		got := c.Out()
		if got == nil {
			t.Fatal("Expected Out() to return non-nil channel")
		}

		// Test that we can read from it
		go func() {
			outChan <- "test message"
		}()

		msg := <-got
		if msg != "test message" {
			t.Errorf("Expected to read 'test message', got %s", msg)
		}
	})
}

func Test_channel_Err(t *testing.T) {
	t.Run("returns error channel for reading", func(t *testing.T) {
		errChan := make(chan error)
		c := &channel{
			name:   "test",
			output: make(chan string),
			err:    errChan,
			ctx:    context.Background(),
		}

		got := c.Err()
		if got == nil {
			t.Fatal("Expected Err() to return non-nil channel")
		}

		// Test that we can read from it
		testErr := context.Canceled
		go func() {
			errChan <- testErr
		}()

		err := <-got
		if err != testErr {
			t.Errorf("Expected to read context.Canceled, got %v", err)
		}
	})
}

func Test_channel_Start(t *testing.T) {
	t.Run("starts without semaphore", func(t *testing.T) {
		c := &channel{
			name:   "test",
			output: make(chan string),
			err:    make(chan error),
			ctx:    context.Background(),
			sem:    nil,
			wg:     nil,
		}

		err := c.Start(1)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		c.Close()

		// Verify channels are closed
		select {
		case _, ok := <-c.Out():
			if ok {
				t.Error("Expected output channel to be closed")
			}
		default:
			t.Error("Expected to be able to read from closed channel")
		}
	})

	t.Run("starts with semaphore successfully", func(t *testing.T) {
		sem := semaphore.NewWeighted(10)
		c := &channel{
			name:   "test",
			output: make(chan string),
			err:    make(chan error),
			ctx:    context.Background(),
			sem:    sem,
			wg:     nil,
		}

		err := c.Start(2)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Verify semaphore was acquired (8 left out of 10)
		if !sem.TryAcquire(8) {
			t.Error("Expected to be able to acquire 8 more")
		}
		sem.Release(8)

		c.Close()

		// Verify semaphore was released (should have full 10 now)
		if !sem.TryAcquire(10) {
			t.Error("Expected semaphore to be fully released")
		}
		sem.Release(10)
	})

	t.Run("starts with waitgroup", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(1)

		c := &channel{
			name:   "test",
			output: make(chan string),
			err:    make(chan error),
			ctx:    context.Background(),
			sem:    nil,
			wg:     &wg,
		}

		err := c.Start(1)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		c.Close()

		// Create a channel that closes when waitgroup is done
		doneCh := make(chan struct{})
		go func() {
			wg.Wait()
			close(doneCh)
		}()

		// Wait for the channel to close or timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		select {
		case <-doneCh:
			// Success - waitgroup was decremented
		case <-ctx.Done():
			t.Error("Waitgroup was not decremented in time")
		}
	})

	t.Run("handles zero weight", func(t *testing.T) {
		sem := semaphore.NewWeighted(10)
		c := &channel{
			name:   "test",
			output: make(chan string),
			err:    make(chan error),
			ctx:    context.Background(),
			sem:    sem,
			wg:     nil,
		}

		err := c.Start(0)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Should default to weight 1
		if !sem.TryAcquire(9) {
			t.Error("Expected weight to default to 1")
		}
		sem.Release(9)

		c.Close()
	})

	t.Run("handles negative weight", func(t *testing.T) {
		sem := semaphore.NewWeighted(10)
		c := &channel{
			name:   "test",
			output: make(chan string),
			err:    make(chan error),
			ctx:    context.Background(),
			sem:    sem,
			wg:     nil,
		}

		err := c.Start(-5)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// Should default to weight 1
		if !sem.TryAcquire(9) {
			t.Error("Expected weight to default to 1")
		}
		sem.Release(9)

		c.Close()
	})

	t.Run("handles cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		sem := semaphore.NewWeighted(10)
		c := &channel{
			name:   "test",
			output: make(chan string),
			err:    make(chan error),
			ctx:    ctx,
			sem:    sem,
			wg:     nil,
		}

		err := c.Start(1)
		if err == nil {
			t.Error("Expected error due to cancelled context")
		}

		// Calling Close should not panic and should not release (since acquire failed)
		c.Close()

		// Semaphore should still have full capacity
		if !sem.TryAcquire(10) {
			t.Error("Expected semaphore to not be acquired when context is cancelled")
		}
		sem.Release(10)
	})
}
