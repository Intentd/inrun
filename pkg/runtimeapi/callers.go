package runtimeapi

import (
	"net/url"
)

// IsUnique lists every existing instance of the CRD via the runtime's cache
// and reports whether none of them (other than the CR under evaluation
// itself) has field == value.
func (q *runtimeQuery) IsUnique(field, value, selfNamespace, selfName string) (bool, error) {
	reqURL := q.endpoint + CRListPath(q.crdName) + "?field=" + url.QueryEscape(field)

	var list CRListResponse
	_, err := q.result(list, "uniqueness", reqURL)
	if err != nil {
		return false, err
	}

	for _, item := range list.Items {
		if item.Namespace == selfNamespace && item.Name == selfName {
			continue // never a duplicate of its own stored value
		}
		if item.Value == value {
			return false, nil
		}
	}
	return true, nil
}

// Health returns the /health endpoint of the CRD via the runtime's /catalog endpoint
// The value is added to the resolver for validation and mutation rules to gate on
func (q *runtimeQuery) Health() map[string]interface{} {
	reqURL := q.endpoint + HealthPath(q.crdName)

	var health map[string]interface{}
	_, err := q.result(health, "health", reqURL)
	if err != nil {
		return map[string]interface{}{}
	}
	return health
}

// Metrics returns the /crdName endpoint of the CRD via the runtime's /catalog endpoint
// The value is added to the resolver for validation and mutation rules to gate on
func (q *runtimeQuery) Metrics() map[string]interface{} {
	reqURL := q.endpoint + CRDPath(q.crdName)

	// Parse top-level "metrics" key from the /catalog/{crd} response.
	var m struct {
		Metrics map[string]interface{} `json:"metrics"`
	}

	_, err := q.result(m, "metrics", reqURL)
	if err != nil {
		return map[string]interface{}{}
	}

	return m.Metrics
}
