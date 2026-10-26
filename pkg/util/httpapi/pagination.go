// Pagination primitives for the Gateway API list endpoints.
// Kept deliberately simple for v1 — offset/limit with an optional continue
// token for Kubernetes list continuation.
// ─────────────────────────────────────────────────────────────────────────────

package httpapi

import (
	"net/http"
	"strconv"
)

const (
	// DefaultPageLimit is the number of items returned when the caller does
	// not supply a limit. Matches kubectl's default page size.
	DefaultPageLimit = 100

	// MaxPageLimit caps the limit to prevent callers from requesting
	// unbounded result sets.
	MaxPageLimit = 1000
)

// PaginationParams holds the parsed pagination values for a single request.
type PaginationParams struct {
	// Limit is the maximum number of items to return. Always in [1, MaxPageLimit].
	Limit int

	// Offset is the zero-based index of the first item to return.
	Offset int

	// Continue is an opaque continuation token returned by Kubernetes list
	// calls. When non-empty it takes precedence over Offset for server-side
	// pagination via the Kubernetes API.
	Continue string
}

// ParsePagination extracts limit, offset, and continue from the request query.
// Invalid or missing values fall back to safe defaults — this function never
// returns an error; callers do not need to validate the result.
func ParsePagination(r *http.Request) PaginationParams {
	p := PaginationParams{
		Limit:    DefaultPageLimit,
		Continue: r.URL.Query().Get("continue"),
	}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			p.Limit = n
		}
	}
	if p.Limit > MaxPageLimit {
		p.Limit = MaxPageLimit
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			p.Offset = n
		}
	}

	return p
}

// PageItems slices a string slice according to pagination params and returns
// the page and the total count. Out-of-bounds offsets return an empty slice.
//
// Used for service-list and schema list responses where items are already in memory.
// For Kubernetes resource lists, use the continue token with the dynamic client.
func PageItems[T any](items []T, p PaginationParams) (page []T, total int) {
	total = len(items)
	start := p.Offset
	if start > total {
		start = total
	}
	end := start + p.Limit
	if end > total {
		end = total
	}
	return items[start:end], total
}

// PaginatedResponse is the envelope for paginated list responses.
type PaginatedResponse[T any] struct {
	// Total is the total number of items available (before pagination).
	Total int `json:"total"`

	// Limit is the maximum items per page as requested.
	Limit int `json:"limit"`

	// Offset is the index of the first item in this page.
	Offset int `json:"offset"`

	// Continue is the Kubernetes continuation token for the next page.
	// Present only when the underlying list has more items beyond this page.
	Continue string `json:"continue,omitempty"`

	// Items is the current page of results.
	Items []T `json:"items"`
}
