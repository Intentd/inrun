package note

import (
	"strings"
	"text/template"
)

// podNotes registers helpers for inspecting enriched pod lists embedded by
// children.go under the reserved "_pods" key.
//
// Usage:
//
//	tmpl.Funcs(note.podNotes())
//
// Template examples:
//
//	"{{ podNames .children.deployment }}"        → "web-abc, web-def"
//	"{{ podIPs .children.deployment }}"          → "10.0.0.1, 10.0.0.2"
//	"{{ podCount .children.deployment }}"        → 2
//	"{{ readyPodCount .children.deployment }}"   → 2
//	"{{ hasCrashingPod .children.deployment }}"  → false
//
// These notes require pod enrichment to be enabled for the CRD:
//
//	enrich: [pods]   # or enrichAll: true
//
// Without enrichment the "_pods" key is absent and all notes return their
// zero values ("", 0, false) — the same safe fallback as any missing field.
func podNotes() template.FuncMap {
	return template.FuncMap{
		"podNames":       notePodNames,
		"podIPs":         notePodIPs,
		"podPhases":      notePodPhases,
		"podNodes":       notePodNodes,
		"podCount":       notePodCount,
		"readyPodCount":  noteReadyPodCount,
		"podMaxRestarts": notePodMaxRestarts,
		"hasCrashingPod": noteHasCrashingPod,
		"podByOrdinal":   notePodByOrdinal,
		// Container status notes — navigate containers[] within each pod summary.
		"podContainerReasons":             notePodContainerReasons,
		"podContainerState":               notePodContainerState,
		"podCrashLoopDetected":            notePodCrashLoopDetected,
		"podImagePullBackOffDetected":     notePodImagePullBackOffDetected,
		"podErrImagePullDetected":         notePodErrImagePullDetected,
		"podErrorDetected":                notePodErrorDetected,
		"podOOMKilledDetected":            notePodOOMKilledDetected,
		"podRunContainerErrorDetected":    notePodRunContainerErrorDetected,
		"podCreateContainerErrorDetected": notePodCreateContainerErrorDetected,
		"podInvalidImageNameDetected":     notePodInvalidImageNameDetected,
		"podPreStartHookErrorDetected":    notePodPreStartHookErrorDetected,
		"podPostStartHookErrorDetected":   notePodPostStartHookErrorDetected,
	}
}

// notePodNames returns a comma-separated list of pod names owned by the
// enriched resource. Returns "" when no pods are present.
//
//	{{ podNames .children.deployment }}  → "web-abc, web-def"
func notePodNames(obj interface{}) string {
	return joinPodField(obj, "name")
}

// notePodIPs returns a comma-separated list of pod IP addresses.
// Returns "" when no pods are present or IPs have not yet been assigned.
//
//	{{ podIPs .children.deployment }}  → "10.0.0.1, 10.0.0.2"
func notePodIPs(obj interface{}) string {
	return joinPodField(obj, "ip")
}

// notePodCount returns the total number of pods owned by the enriched resource.
//
//	{{ podCount .children.deployment }}  → 3
func notePodCount(obj interface{}) int {
	return len(getPods(obj))
}

// noteReadyPodCount returns the number of pods whose Ready condition is True.
//
//	{{ readyPodCount .children.deployment }}  → 2
func noteReadyPodCount(obj interface{}) int {
	pods := getPods(obj)
	count := 0
	for _, p := range pods {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if ready, _ := pod["ready"].(bool); ready {
			count++
		}
	}
	return count
}

// noteHasCrashingPod returns true when any pod shows signs of being unhealthy:
// restart count above 2, or any container has a known failure reason.
//
//	{{ hasCrashingPod .children.deployment }}  → false
func noteHasCrashingPod(obj interface{}) bool {
	pods := getPods(obj)
	badReasons := []string{
		"CrashLoopBackOff", "OOMKilled", "Error",
		"ImagePullBackOff", "ErrImagePull", "InvalidImageName",
		"RunContainerError", "CreateContainerError",
		"PreStartHookError", "PostStartHookError",
	}
	for _, reason := range badReasons {
		if hasContainerReason(pods, reason) {
			return true
		}
	}
	for _, p := range pods {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		switch v := pod["restartCount"].(type) {
		case int64:
			if v > 2 {
				return true
			}
		case float64:
			if v > 2 {
				return true
			}
		case int:
			if v > 2 {
				return true
			}
		}
	}
	return false
}

// notePodPhases returns a comma-separated string of pod phases in order.
// Useful for surfacing the phase distribution of a StatefulSet or Deployment.
//
//	{{ podPhases .children.statefulset }}  → "Running, Running, Pending"
func notePodPhases(obj interface{}) string {
	return joinPodField(obj, "phase")
}

// notePodNodes returns a comma-separated list of node names the pods are
// scheduled on. Returns "" when pods are pending (not yet scheduled).
//
//	{{ podNodes .children.deployment }}  → "node-1, node-2"
func notePodNodes(obj interface{}) string {
	return joinPodField(obj, "node")
}

// notePodMaxRestarts returns the highest restart count across all pods.
// Zero when no pods are present or none have restarted.
//
//	{{ podMaxRestarts .children.deployment }}  → 3
func notePodMaxRestarts(obj interface{}) int64 {
	var max int64
	for _, p := range getPods(obj) {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		var v int64
		switch r := pod["restartCount"].(type) {
		case int64:
			v = r
		case float64:
			v = int64(r)
		case int:
			v = int64(r)
		}
		if v > max {
			max = v
		}
	}
	return max
}

// notePodByOrdinal returns the pod summary at the given ordinal index.
// Ordinal is embedded by children.go from the pod name suffix (StatefulSet pods
// are named <name>-0, <name>-1, etc.). Returns nil when no pod matches.
//
//	{{ podByOrdinal .children.statefulset 0 }}   → map with name, ip, phase, ...
//	{{ (podByOrdinal .children.statefulset 0).ip }}  → "10.0.0.1"
func notePodByOrdinal(obj interface{}, ordinal int64) interface{} {
	for _, p := range getPods(obj) {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		var o int64
		switch v := pod["ordinal"].(type) {
		case int64:
			o = v
		case float64:
			o = int64(v)
		case int:
			o = int64(v)
		}
		if o == ordinal {
			return pod
		}
	}
	return nil
}

// ── Container status notes ────────────────────────────────────────────────

// notePodCrashLoopDetected returns true when any container is in CrashLoopBackOff.
//
//	{{ podCrashLoopDetected .children.deployment }}
func notePodCrashLoopDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "CrashLoopBackOff")
}

// notePodImagePullBackOffDetected returns true when any container has reason ImagePullBackOff.
//
//	{{ podImagePullBackOffDetected .children.deployment }}
//	→ false
func notePodImagePullBackOffDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "ImagePullBackOff")
}

// notePodErrImagePullDetected returns true when any container has reason ErrImagePull.
//
//	{{ podErrImagePullDetected .children.deployment }}
//	→ false
func notePodErrImagePullDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "ErrImagePull")
}

// notePodErrorDetected returns true when any container has reason Error.
//
//	{{ podErrorDetected .children.deployment }}
//	→ false
func notePodErrorDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "Error")
}

// notePodOOMKilledDetected returns true when any container has reason OOMKilled.
//
//	{{ podOOMKilledDetected .children.deployment }}
//	→ true
func notePodOOMKilledDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "OOMKilled")
}

// notePodRunContainerErrorDetected returns true when any container has reason RunContainerError.
//
//	{{ podRunContainerErrorDetected .children.deployment }}
//	→ false
func notePodRunContainerErrorDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "RunContainerError")
}

// notePodCreateContainerErrorDetected returns true when any container has reason CreateContainerError.
//
//	{{ podCreateContainerErrorDetected .children.deployment }}
//	→ false
func notePodCreateContainerErrorDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "CreateContainerError")
}

// notePodInvalidImageNameDetected returns true when any container has reason InvalidImageName.
//
//	{{ podInvalidImageNameDetected .children.deployment }}
//	→ false
func notePodInvalidImageNameDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "InvalidImageName")
}

// notePodPreStartHookErrorDetected returns true when any container has reason PreStartHookError.
//
//	{{ podPreStartHookErrorDetected .children.deployment }}
//	→ false
func notePodPreStartHookErrorDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "PreStartHookError")
}

// notePodPostStartHookErrorDetected returns true when any container has reason PostStartHookError.
//
//	{{ podPostStartHookErrorDetected .children.deployment }}
//	→ false
func notePodPostStartHookErrorDetected(obj interface{}) bool {
	return hasContainerReason(getPods(obj), "PostStartHookError")
}

// notePodContainerReasons returns a comma-separated list of unique waiting or
// terminated reasons across all containers in all pods. Empty reasons are omitted.
// Useful for surfacing ImagePullBackOff, CrashLoopBackOff, OOMKilled, etc.
// Requires enrich: [pods] on the CRD.
//
//	{{ podContainerReasons .children.deployment }}
//	→ "CrashLoopBackOff, ImagePullBackOff"
func notePodContainerReasons(obj interface{}) string {
	seen := map[string]bool{}
	var parts []string
	for _, p := range getPods(obj) {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		containers, _ := pod["containers"].([]interface{})
		for _, c := range containers {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			r, _ := cm["reason"].(string)
			if r != "" && !seen[r] {
				seen[r] = true
				parts = append(parts, r)
			}
		}
	}
	return strings.Join(parts, ", ")
}

// notePodContainerState returns the state of a named container within the pod
// at the given ordinal. Returns "" when the pod or container is not found.
// Requires enrich: [pods] on the CRD.
//
//	{{ podContainerState .children.statefulset 0 "app" }}  → "Running"
//	{{ podContainerState .children.statefulset 0 "app" }}  → "Waiting"
func notePodContainerState(obj interface{}, ordinal int64, containerName string) string {
	pod := notePodByOrdinal(obj, ordinal)
	if pod == nil {
		return ""
	}
	pm, ok := pod.(map[string]interface{})
	if !ok {
		return ""
	}
	containers, _ := pm["containers"].([]interface{})
	for _, c := range containers {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if n, _ := cm["name"].(string); n == containerName {
			state, _ := cm["state"].(string)
			return state
		}
	}
	return ""
}

// ── internal helpers ──────────────────────────────────────────────────────

// getPods navigates the "_pods" key of an enriched resource map.
// Returns nil when the key is absent or enrichment was not enabled.
func getPods(obj interface{}) []interface{} {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}
	pods, _ := m["_pods"].([]interface{})
	return pods
}

// joinPodField collects field from every pod in _pods and joins with ", ".
func joinPodField(obj interface{}, field string) string {
	pods := getPods(obj)
	if len(pods) == 0 {
		return ""
	}
	parts := make([]string, 0, len(pods))
	for _, p := range pods {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if v, _ := pod[field].(string); v != "" {
			parts = append(parts, v)
		}
	}
	return strings.Join(parts, ", ")
}

// hasContainerReason returns true if any container across any pod has the given reason.
func hasContainerReason(pods []interface{}, targetReason string) bool {
	for _, p := range pods {
		pod, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		containers, _ := pod["containers"].([]interface{})
		for _, c := range containers {
			cm, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			if r, _ := cm["reason"].(string); r == targetReason {
				return true
			}
		}
	}
	return false
}
