package runtimeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// query GETs url and returns the response when the runtime answered 200. The
// caller closes its body.
func (q *runtimeQuery) query(url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(q.ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building runtime request: %w", err)
	}
	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying runtime: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("runtime returned %d", resp.StatusCode)
	}
	return resp, nil
}

// result decodes the runtime's JSON answer at url into out, a pointer.
func (q *runtimeQuery) result(out any, caller, url string) error {
	resp, err := q.query(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding %s response: %w", caller, err)
	}
	return nil
}
