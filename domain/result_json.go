package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// The JSON form of Result is what remote reconcilers return. Durations are
// strings ("60s"):
//
//	{"requeue": {"after": "60s", "reason": "…"}, "requeueAfter": "60s",
//	 "cleanup": {"after": "15s", "reason": "…"}, "status": {…}, "resources": […],
//	 "labels": {…}, "annotations": {…}}

type resultJSON struct {
	Requeue      *Requeue                 `json:"requeue,omitempty"`
	RequeueAfter string                   `json:"requeueAfter,omitempty"`
	Cleanup      *Cleanup                 `json:"cleanup,omitempty"`
	Status       map[string]interface{}   `json:"status,omitempty"`
	Resources    []map[string]interface{} `json:"resources,omitempty"`
	Labels       map[string]string        `json:"labels,omitempty"`
	Annotations  map[string]string        `json:"annotations,omitempty"`
}

type delayJSON struct {
	After  string `json:"after,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// MarshalJSON writes durations as strings.
func (r Result) MarshalJSON() ([]byte, error) {
	out := resultJSON{
		Requeue:     r.Requeue,
		Cleanup:     r.Cleanup,
		Status:      r.Status,
		Resources:   r.Resources,
		Labels:      r.Labels,
		Annotations: r.Annotations,
	}
	if r.RequeueAfter > 0 {
		out.RequeueAfter = r.RequeueAfter.String()
	}
	return json.Marshal(out)
}

// UnmarshalJSON reads durations from strings and rejects malformed or negative ones.
func (r *Result) UnmarshalJSON(b []byte) error {
	var in resultJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	out := Result{Requeue: in.Requeue, Cleanup: in.Cleanup, Status: in.Status, Resources: in.Resources, Labels: in.Labels, Annotations: in.Annotations}
	if in.RequeueAfter != "" {
		d, err := parseDelay("requeueAfter", in.RequeueAfter)
		if err != nil {
			return err
		}
		out.RequeueAfter = d
	}
	*r = out
	return nil
}

// MarshalJSON writes After as a string.
func (q Requeue) MarshalJSON() ([]byte, error) {
	return json.Marshal(delayJSON{After: q.After.String(), Reason: q.Reason})
}

// UnmarshalJSON reads After from a string; it is required.
func (q *Requeue) UnmarshalJSON(b []byte) error {
	var in delayJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	if in.After == "" {
		return fmt.Errorf("requeue.after is required")
	}
	d, err := parseDelay("requeue.after", in.After)
	if err != nil {
		return err
	}
	*q = Requeue{After: d, Reason: in.Reason}
	return nil
}

// MarshalJSON writes After as a string, omitted when zero (now).
func (c Cleanup) MarshalJSON() ([]byte, error) {
	out := delayJSON{Reason: c.Reason}
	if c.After > 0 {
		out.After = c.After.String()
	}
	return json.Marshal(out)
}

// UnmarshalJSON reads After from a string; empty means now.
func (c *Cleanup) UnmarshalJSON(b []byte) error {
	var in delayJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	out := Cleanup{Reason: in.Reason}
	if in.After != "" {
		d, err := parseDelay("cleanup.after", in.After)
		if err != nil {
			return err
		}
		out.After = d
	}
	*c = out
	return nil
}

func parseDelay(field, v string) (time.Duration, error) {
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: %w", field, v, err)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s %q must not be negative", field, v)
	}
	return d, nil
}
