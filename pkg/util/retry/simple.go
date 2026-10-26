package retry

import (
	"errors"
	"math/rand/v2"
	"time"
)

// Options sets how many times Retry and RetryBackoff call fn, and the delay
// between calls.
type Options struct {
	Attempts int
	Delay    time.Duration
}

func Retry(fn func() error, opts Options) error {
	delay := opts.Delay
	attempts := opts.Attempts

	if attempts < 1 {
		return errors.New("attempts must be >= 1")
	}

	for i := 1; i <= attempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		if i == attempts {
			return err
		}

		time.Sleep(delay)
	}

	return nil
}

func Jitter(d time.Duration) time.Duration {
	// ±50% jitter
	j := rand.Float64()*float64(d) - float64(d)/2
	return d + time.Duration(j)
}

func RetryBackoff(fn func() error, opts Options) error {
	delay := opts.Delay
	attempts := opts.Attempts

	for i := 1; i <= attempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		if i == attempts {
			return err
		}

		time.Sleep(Jitter(delay))
		delay *= 2
	}

	return nil
}
