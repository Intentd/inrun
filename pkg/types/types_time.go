package types

import (
	"fmt"
	"time"

	"github.com/intentd/kel/core"
	"github.com/intentd/tools/duration"
	"github.com/intentd/tools/retry"
	"gopkg.in/yaml.v3"
)

// Duration is a time.Duration that unmarshals from YAML strings — Go's units
// ("15s", "2m", "1h") plus d/w/mo/y ("10d", "2w", "3mo", "1y").
type Duration = duration.Duration

// RetryBackoffConfig is declared under queue.retryBackoff or external[].retryBackoff.
// Shorthand: a plain duration string sets Initial only.
// Full form: set Initial, Max, Multiplier, and MaxAttempts individually.
type RetryBackoffConfig struct {
	// Initial is the first backoff delay. Default: 500ms.
	Initial Duration `yaml:"initial,omitempty" json:"initial,omitempty"`
	// Max caps the delay so it does not grow unboundedly. Default: 30s.
	Max Duration `yaml:"max,omitempty" json:"max,omitempty"`
	// Multiplier scales the delay after each attempt. Default: 2.0.
	Multiplier float64 `yaml:"multiplier,omitempty" json:"multiplier,omitempty"`
	// MaxAttempts is the total number of calls including the first. Default: 3.
	MaxAttempts int `yaml:"maxAttempts,omitempty" json:"maxAttempts,omitempty"`
}

// UnmarshalYAML allows RetryBackoffConfig to be written as either a plain duration
// string (shorthand for initial only) or the full struct form.
//
//	retryBackoff: 5s                    # shorthand — initial: 5s, defaults for the rest
//	retryBackoff:                       # full form
//	  initial: 100ms
//	  max: 10m
//	  multiplier: 2.0
//	  maxAttempts: 5
func (r *RetryBackoffConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		d, err := core.ParseTimeDuration(value.Value)
		if err != nil {
			return fmt.Errorf("retryBackoff: %w", err)
		}
		r.Initial = Duration{Duration: d}
		return nil
	}
	// Full struct form — avoid infinite recursion with alias type.
	type plain RetryBackoffConfig
	return value.Decode((*plain)(r))
}

// ToRetryDoOptions converts the declaration into retry.DoOptions, applying defaults.
func (r *RetryBackoffConfig) ToRetryDoOptions() retry.DoOptions {
	if r == nil {
		return retry.DoOptions{}
	}
	return retry.DoOptions{
		Base:        r.Initial.Duration,
		Max:         r.Max.Duration,
		Multiplier:  r.Multiplier,
		MaxAttempts: r.MaxAttempts,
	}
}

// WorstCaseDuration returns the maximum wall time a full retry sequence can
// take, assuming no jitter. Used by the validator to compare against resync.
func (r *RetryBackoffConfig) WorstCaseDuration() time.Duration {
	opts := r.ToRetryDoOptions()
	opts.ApplyDefaults()
	delay := opts.Base
	var total time.Duration
	for i := 1; i < opts.MaxAttempts; i++ {
		total += delay
		next := time.Duration(float64(delay) * opts.Multiplier)
		if next > opts.Max {
			next = opts.Max
		}
		delay = next
	}
	return total
}
