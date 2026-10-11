package types

import (
	"fmt"
	"time"

	"github.com/intentd/inrun/pkg/utils"
	"gopkg.in/yaml.v3"
)

// TimeWindow declares a clock-based active window.
type TimeWindow struct {
	// After — active after this time (format: "HH:MM" in 24h).
	After string `yaml:"after,omitempty" json:"after,omitempty"`

	// Before — active before this time (format: "HH:MM" in 24h).
	Before string `yaml:"before,omitempty" json:"before,omitempty"`
}

// DayOfWeekCondition declares which days the condition is active.
// Exactly one of Weekday, Weekend, In, or NotIn must be set.
type DayOfWeekCondition struct {
	// Weekday — active Mon–Fri. Shorthand for in: [Monday..Friday].
	Weekday *bool `yaml:"weekday,omitempty" json:"weekday,omitempty"`

	// Weekend — active Sat–Sun. Shorthand for in: [Saturday, Sunday].
	Weekend *bool `yaml:"weekend,omitempty" json:"weekend,omitempty"`

	// In — active on these days. Full English names: Monday, Tuesday, etc.
	In []string `yaml:"in,omitempty" json:"in,omitempty"`

	// NotIn — active on all days except these.
	NotIn []string `yaml:"notIn,omitempty" json:"notIn,omitempty"`
}

// Duration is a time.Duration that unmarshals from YAML strings — Go's units
// ("15s", "2m", "1h") plus d/w/mo/y ("10d", "2w", "3mo", "1y").
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	if s == "" {
		d.Duration = 0
		return nil
	}
	parsed, err := utils.ParseTimeDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

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
		d, err := utils.ParseTimeDuration(value.Value)
		if err != nil {
			return fmt.Errorf("retryBackoff: %w", err)
		}
		r.Initial = Duration{d}
		return nil
	}
	// Full struct form — avoid infinite recursion with alias type.
	type plain RetryBackoffConfig
	return value.Decode((*plain)(r))
}

// ToRetryDoOptions converts the declaration into utils.RetryDoOptions, applying defaults.
func (r *RetryBackoffConfig) ToRetryDoOptions() utils.RetryDoOptions {
	if r == nil {
		return utils.RetryDoOptions{}
	}
	return utils.RetryDoOptions{
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
