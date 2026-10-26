// pkg/autoscaler/autoscale_semaphore.go
//
// ResizableSemaphore — a weighted semaphore whose capacity can be changed at runtime.
//
// The worker pool in Inrun is gated by this semaphore rather than being a
// fixed goroutine count. All worker goroutines run continuously in a loop;
// the semaphore controls how many may enter the reconcile section simultaneously.
//
// Increasing capacity: new goroutines can acquire immediately.
// Decreasing capacity: goroutines currently inside reconcile finish their
// current work. Goroutines blocked at Acquire wait until capacity is available.
// In-flight reconciles are never interrupted.
//
// This is the correct primitive for operator autoscaling because:
//   - Resize is O(1) — no goroutine creation or cancellation
//   - In-flight reconciles complete cleanly after a scale-down
//   - The semaphore is the single source of truth for concurrency
package autoscaler

import (
	"context"
	"sync"
)

// ResizableSemaphore is a counting semaphore whose capacity can be changed
// at runtime without interrupting goroutines currently holding a token.
type ResizableSemaphore struct {
	mu      sync.Mutex
	cap     int
	current int
	waiters []chan struct{}
}

// NewResizableSemaphore returns a semaphore with the given initial capacity.
func NewResizableSemaphore(capacity int) *ResizableSemaphore {
	if capacity < 1 {
		capacity = 1
	}
	return &ResizableSemaphore{cap: capacity}
}

// Acquire acquires one token, blocking until one is available or ctx is done.
// Returns ctx.Err() if the context is cancelled while waiting.
func (s *ResizableSemaphore) Acquire(ctx context.Context) error {
	s.mu.Lock()

	if s.current < s.cap {
		s.current++
		s.mu.Unlock()
		return nil
	}

	// No capacity — register a waiter channel
	ch := make(chan struct{}, 1)
	s.waiters = append(s.waiters, ch)
	s.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		// Remove from waiters to avoid a goroutine leak
		s.mu.Lock()
		for i, w := range s.waiters {
			if w == ch {
				s.waiters = append(s.waiters[:i], s.waiters[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
		return ctx.Err()
	}
}

// Release releases one token and notifies a waiter if any are queued.
func (s *ResizableSemaphore) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.waiters) > 0 {
		// Hand the token directly to the first waiter — do not decrement current
		ch := s.waiters[0]
		s.waiters = s.waiters[1:]
		ch <- struct{}{}
		return
	}

	s.current--
}

// Resize changes the semaphore capacity.
//
// Scale up (newCap > old): immediately notifies waiting goroutines up to the
// new capacity. Each notification allows one waiter to proceed.
//
// Scale down (newCap < old): the new capacity takes effect immediately for
// new Acquire calls. Goroutines already holding tokens complete their work
// normally — they are not interrupted. The effective concurrency converges
// to newCap as holders release.
//
// Resize is safe to call concurrently with Acquire and Release.
func (s *ResizableSemaphore) Resize(newCap int) {
	if newCap < 1 {
		newCap = 1
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	old := s.cap
	s.cap = newCap

	if newCap <= old {
		return // scale-down: no immediate action needed
	}

	// Scale-up: notify waiters for each new slot
	added := newCap - old
	for added > 0 && len(s.waiters) > 0 {
		ch := s.waiters[0]
		s.waiters = s.waiters[1:]
		s.current++
		ch <- struct{}{}
		added--
	}
}

// Capacity returns the current capacity.
func (s *ResizableSemaphore) Capacity() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cap
}

// Current returns the current number of tokens.
func (s *ResizableSemaphore) Current() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// InFlight returns the number of tokens currently held (goroutines inside reconcile).
func (s *ResizableSemaphore) InFlight() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// BusyPercent returns the percentage of capacity currently in use.
func (s *ResizableSemaphore) BusyPercent() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cap == 0 {
		return 0
	}
	return float64(s.current) / float64(s.cap) * 100
}

// IdlePercent returns the percentage of capacity currently available.
func (s *ResizableSemaphore) IdlePercent() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cap == 0 {
		return 0
	}
	idle := s.cap - s.current
	if idle < 0 {
		idle = 0
	}
	return float64(idle) / float64(s.cap) * 100
}
