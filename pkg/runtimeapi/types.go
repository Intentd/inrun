package runtimeapi

// CRListResponse is returned by GET /blueprint/{crd}/cr.
type CRListResponse struct {
	CRD      string      `json:"crd"`
	GVK      string      `json:"gvk"`
	Total    int         `json:"total"`
	Items    []CRSummary `json:"items"`
	IsLeader bool        `json:"isLeader"`
}

// CRSummary is one row in the CR list — equivalent to one line of kubectl get.
type CRSummary struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	Phase       string `json:"phase,omitempty"` // from status.phase if declared
	Ready       bool   `json:"ready"`           // from status.conditions[type=Ready]
	ReadyReason string `json:"readyReason,omitempty"`
	Age         string `json:"age"`
	Generation  int64  `json:"generation"`

	// Value is the resolved value of the ?field= dot-path requested on this
	// list call — e.g. ?field=spec.domain. Empty when no field was
	// requested, or when the CR doesn't have that field. This is what lets
	// unique: true check other instances without a live List() call
	// against the API server: the informer cache already holds full spec
	// data, this just surfaces one field of it instead of discarding
	// everything but the summary. See NewClient.
	Value string `json:"value,omitempty"`
}
