package retry

import (
	"context"
	"time"
)

const DefaultAttempts = 3

// DoOptions controls the behaviour of Do.
type DoOptions struct {
	// MaxAttempts is the total number of calls to fn (including the first). Default: 3.
	MaxAttempts int
	// Base is the initial backoff duration. Default: 500ms.
	Base time.Duration
	// Max caps the backoff so it does not grow unboundedly. Default: 30s.
	Max time.Duration
	// Multiplier scales the delay after each failed attempt. Default: 2.0.
	Multiplier float64
	// Retryable, when set, is called on every error. Returning false stops
	// retrying immediately and returns the error to the caller. nil means
	// every error is retryable.
	Retryable func(error) bool
}

func (o *DoOptions) ApplyDefaults() {
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = DefaultAttempts
	}
	if o.Base <= 0 {
		o.Base = 500 * time.Millisecond
	}
	if o.Max <= 0 {
		o.Max = 30 * time.Second
	}
	if o.Multiplier <= 0 {
		o.Multiplier = 2.0
	}
}

// Do calls fn up to opts.MaxAttempts times with exponential backoff and ±50%
// jitter between attempts. ctx cancellation is honoured between attempts. If
// opts.Retryable is set and returns false the error is returned immediately
// without further attempts.
func Do[T any](ctx context.Context, opts DoOptions, fn func(ctx context.Context) (T, error)) (T, error) {
	opts.ApplyDefaults()
	delay := opts.Base
	var zero T
	for attempt := 1; attempt <= opts.MaxAttempts; attempt++ {
		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		if opts.Retryable != nil && !opts.Retryable(err) {
			return zero, err
		}
		if attempt == opts.MaxAttempts {
			return zero, err
		}
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(Jitter(delay)):
		}
		next := time.Duration(float64(delay) * opts.Multiplier)
		if next > opts.Max {
			next = opts.Max
		}
		delay = next
	}
	return zero, nil
}
