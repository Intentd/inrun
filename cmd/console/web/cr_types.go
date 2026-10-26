package web

import (
	"encoding/json"
	"sort"
)

// ─────────────────────────────────────────────────────────────────────────────
// CR types — mirror the JSON shapes returned by Inrun's
// /blueprint/{crd}/cr and /blueprint/{crd}/cr/{namespace}/{name} endpoints.
// ─────────────────────────────────────────────────────────────────────────────

// CRSummary is one row in the CR list — one line of kubectl get.
type CRSummary struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace,omitempty"`
	Namespaced  bool   `json:"namespaced"`
	Phase       string `json:"phase,omitempty"`
	Ready       bool   `json:"ready"`
	ReadyReason string `json:"readyReason,omitempty"`
	Age         string `json:"age"`
	Generation  int64  `json:"generation"`
}

// CRListResponse is returned by GET /blueprint/{crd}/cr.
type CRListResponse struct {
	CRD      string      `json:"crd"`
	GVK      string      `json:"gvk"`
	Total    int         `json:"total"`
	Items    []CRSummary `json:"items"`
	IsLeader bool        `json:"isLeader"`
}

// ChildSummary is a condensed view of one child resource.
type ChildSummary struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace,omitempty"`
	Kind      string                 `json:"kind"`
	Status    map[string]interface{} `json:"status,omitempty"`
	Ready     bool                   `json:"ready"`
}

// CRDetailResponse is returned by GET /blueprint/{crd}/cr/{namespace}/{name}.
// Children is map[string]json.RawMessage because each value is either a single
// ChildSummary object or an array of ChildSummary objects (when multiple children
// of the same kind exist).
type CRDetailResponse struct {
	Name              string                     `json:"name"`
	Namespace         string                     `json:"namespace,omitempty"`
	Generation        int64                      `json:"generation"`
	CreationTimestamp string                     `json:"creationTimestamp"`
	Labels            map[string]string          `json:"labels,omitempty"`
	Annotations       map[string]string          `json:"annotations,omitempty"`
	Ready             bool                       `json:"ready"`
	ReadyReason       string                     `json:"readyReason,omitempty"`
	ReadyMessage      string                     `json:"readyMessage,omitempty"`
	Status            map[string]interface{}     `json:"status,omitempty"`
	Children          map[string]json.RawMessage `json:"children,omitempty"`
	EventsEndpoint    string                     `json:"eventsEndpoint"`
	IsLeader          bool                       `json:"isLeader"`
}

// ChildGroup is the normalised view of children grouped by kind.
// IsMultiple is true when there are more than one child of this kind.
type ChildGroup struct {
	Kind       string
	Items      []ChildSummary
	IsMultiple bool
	ReadyCount int // number of ready items — pre-computed for template use
}

func countReady(items []ChildSummary) int {
	n := 0
	for _, s := range items {
		if s.Ready {
			n++
		}
	}
	return n
}

// normalizeChildGroups converts the polymorphic children map into a sorted
// slice of ChildGroup values ready for template rendering.
func normalizeChildGroups(raw map[string]json.RawMessage) []ChildGroup {
	if len(raw) == 0 {
		return nil
	}
	groups := make([]ChildGroup, 0, len(raw))
	for kind, msg := range raw {
		// Try array first (multiple children of same kind)
		var arr []ChildSummary
		if err := json.Unmarshal(msg, &arr); err == nil && len(arr) > 0 {
			sort.Slice(arr, func(i, j int) bool { return arr[i].Name < arr[j].Name })
			groups = append(groups, ChildGroup{Kind: kind, Items: arr, IsMultiple: len(arr) > 1, ReadyCount: countReady(arr)})
			continue
		}
		// Fall back to single object
		var single ChildSummary
		if err := json.Unmarshal(msg, &single); err == nil {
			ready := 0
			if single.Ready {
				ready = 1
			}
			groups = append(groups, ChildGroup{Kind: kind, Items: []ChildSummary{single}, IsMultiple: false, ReadyCount: ready})
		}
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].Kind < groups[j].Kind })
	return groups
}

// CREvent is one Kubernetes event involving this CR.
type CREvent struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	Object    string `json:"object"`
	Count     int32  `json:"count"`
	FirstSeen string `json:"firstSeen"`
	LastSeen  string `json:"lastSeen"`
	Age       string `json:"age"`
}

// CREventsResponse is returned by GET /blueprint/{crd}/cr/{namespace}/{name}/events.
type CREventsResponse struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace,omitempty"`
	Total     int       `json:"total"`
	Events    []CREvent `json:"events"`
}

// ─────────────────────────────────────────────────────────────────────────────
// View models — assembled from API responses for template rendering.
// ─────────────────────────────────────────────────────────────────────────────

// CRListView is the data passed to cr_list.html.
type CRListView struct {
	BlueprintName   string // e.g. "demo"
	Instance        string // Inrun instance URL
	CRDName         string // e.g. "pipeline"
	GVK             string // e.g. "demo.inrun.dev/v1alpha1, Kind=Pipeline"
	Total           int
	Items           []CRSummary
	BackURL         string // back to /blueprint/{crd}
	ServeEnabled    bool   // true when this CRD has serve.enabled: true in the Blueprint
	GatewayEndpoint string // base URL of the companion gateway; empty when no gateway
}

// CRDetailView is the data passed to cr_detail.html.
type CRDetailView struct {
	BlueprintName string // e.g. "demo"
	Instance      string
	CRDName       string
	CR            CRDetailResponse
	Events        []CREvent
	EventTotal    int
	Phase         string       // extracted from status.phase for convenience
	ChildGroups   []ChildGroup // normalised children — single or grouped per kind
	BackURL       string       // back to /blueprint/{crd}/cr
}
