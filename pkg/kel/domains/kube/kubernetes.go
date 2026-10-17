package note

import (
	"fmt"
	"text/template"
)

// Kubernetes-aware helpers for templates.
// These functions safely navigate Kubernetes-style unstructured objects
// (map[string]interface{}) without panics, and provide convenient access
// to metadata, spec, status, ownerReferences, and conditions.
//
//
// Usage examples:
//   {{ meta .children.cronjob | json }}
//   {{ labels .children.cronjob }}
//   {{ annotations .children.cronjob }}
//   {{ spec .children.cronjob }}
//   {{ status .children.cronjob }}
//   {{ get .children.cronjob "status" "lastScheduleTime" }}
//   {{ ownerKind .children.cronjob }}
//   {{ ownerName .children.cronjob }}
//   {{ hasCondition .children.deployment "Available" }}
//   {{ boolTernary (boolDefault (get . "spec" "suspend") false) "Suspended" "Active" }}

func kubernetesNotes() template.FuncMap {
	return template.FuncMap{
		// ── Metadata ──────────────────────────────────────────────────────────
		"meta":        noteMeta,
		"name":        noteName,
		"namespace":   noteNamespace,
		"labels":      noteLabels,
		"annotations": noteAnnotations,

		// ── Single-key label/annotation accessors ─────────────────────────────
		"getLabel":         noteGetLabel,
		"getLabelInt":      noteGetLabelInt,
		"hasLabel":         noteHasLabel,
		"getAnnotation":    noteGetAnnotation,
		"getAnnotationInt": noteGetAnnotationInt,
		"hasAnnotation":    noteHasAnnotation,
		"labelMatches":     noteLabelMatches,

		// ── Spec / Status / Phase ─────────────────────────────────────────────
		"spec":      noteSpec,
		"status":    noteStatus,
		"getStatus": noteGetStatus,
		"hasStatus": noteHasStatus,
		"phase":     notePhase,

		// ── Safe nested field lookup ──────────────────────────────────────────
		"get": noteGet,

		// ── Owner references ──────────────────────────────────────────────────
		"ownerKind": noteOwnerKind,
		"ownerName": noteOwnerName,

		// ── Conditions ────────────────────────────────────────────────────────
		"hasCondition":     noteHasCondition,
		"conditionReason":  noteConditionReason,
		"conditionMessage": noteConditionMessage,

		// ── Existence and lifecycle ───────────────────────────────────────────
		"resourceExists": noteExists,
		"isTerminating":  noteIsTerminating,

		// ── Generation sync ───────────────────────────────────────────────────
		"generation":         noteGeneration,
		"observedGeneration": noteObservedGeneration,
		"isSynced":           noteIsSynced,

		// ── Spec resource request accessors ──────────────────────────────────
		"resourceCPU":    noteResourceCPU,
		"resourceMemory": noteResourceMemory,
	}
}

//
// ────────────────────────────────────────────────────────────────
//   1. METADATA HELPERS
// ────────────────────────────────────────────────────────────────
//

// noteMeta returns the metadata map of a Kubernetes object.
// Safe for nil or missing metadata.
//
//	{{ meta .children.cronjob | json }}
func noteMeta(obj interface{}) map[string]interface{} {
	if m, ok := obj.(map[string]interface{}); ok {
		if meta, ok := m["metadata"].(map[string]interface{}); ok {
			return meta
		}
	}
	return map[string]interface{}{}
}

// noteName returns metadata.name or an empty string.
//
//	{{ name .children.deployment }}  → "my-deployment"
func noteName(obj interface{}) string {
	meta := noteMeta(obj)
	if n, ok := meta["name"].(string); ok {
		return n
	}
	return ""
}

// noteNamespace returns metadata.namespace or an empty string.
//
//	{{ namespace .children.deployment }}  → "inrun-system"
func noteNamespace(obj interface{}) string {
	meta := noteMeta(obj)
	if ns, ok := meta["namespace"].(string); ok {
		return ns
	}
	return ""
}

// noteLabels returns the labels map from metadata.
// Returns an empty map if labels are missing.
//
//	{{ labels .children.cronjob }}
func noteLabels(obj interface{}) map[string]interface{} {
	meta := noteMeta(obj)
	if lbls, ok := meta["labels"].(map[string]interface{}); ok {
		return lbls
	}
	return map[string]interface{}{}
}

// noteAnnotations returns the annotations map from metadata.
// Returns an empty map if annotations are missing.
//
//	{{ annotations .children.cronjob }}
func noteAnnotations(obj interface{}) map[string]interface{} {
	meta := noteMeta(obj)
	if ann, ok := meta["annotations"].(map[string]interface{}); ok {
		return ann
	}
	return map[string]interface{}{}
}

// ── Single-key label/annotation accessors ─────────────────────────────────────

// noteGetLabel returns the value of a single label key. Returns "" when the
// key is absent or the object has no labels.
//
//	{{ getLabel .children.deployment "app.kubernetes.io/name" }} → "my-app"
func noteGetLabel(obj interface{}, key string) string {
	v, _ := noteLabels(obj)[key].(string)
	return v
}

// noteGetLabelInt returns a label value parsed as int64. Returns 0 when the
// key is absent or the value is non-numeric.
//
//	{{ getLabelInt .children.deployment "replica-count" }} → 3
func noteGetLabelInt(obj interface{}, key string) int64 {
	return toInt64(noteLabels(obj)[key])
}

// noteHasLabel returns true when the object has the given label key with a
// non-empty value.
//
//	{{ hasLabel .children.deployment "app.kubernetes.io/managed-by" }}
func noteHasLabel(obj interface{}, key string) bool {
	return noteGetLabel(obj, key) != ""
}

// noteGetAnnotation returns the value of a single annotation key. Returns ""
// when the key is absent or the object has no annotations.
//
//	{{ getAnnotation . "autoscale/min-replicas" | default "2" }}
func noteGetAnnotation(obj interface{}, key string) string {
	v, _ := noteAnnotations(obj)[key].(string)
	return v
}

// noteGetAnnotationInt returns an annotation value parsed as int64. Returns 0
// when the key is absent or the value is non-numeric.
//
//	{{ getAnnotationInt . "autoscale/min-replicas" | default 2 }}
func noteGetAnnotationInt(obj interface{}, key string) int64 {
	return toInt64(noteAnnotations(obj)[key])
}

// noteHasAnnotation returns true when the object has the given annotation
// key with a non-empty value.
//
//	{{ hasAnnotation . "autoscale/enabled" }}
func noteHasAnnotation(obj interface{}, key string) bool {
	return noteGetAnnotation(obj, key) != ""
}

// noteLabelMatches returns true when the object's labels contain every
// key/value pair given as variadic arguments. Extra labels on the object are
// ignored. kvs must have an even number of elements (key, value, key, value...);
// an odd count is treated as a non-match rather than a panic.
//
//	{{ labelMatches .children.deployment "app" "frontend" "env" "prod" }}
func noteLabelMatches(obj interface{}, kvs ...string) bool {
	if len(kvs)%2 != 0 {
		return false
	}
	lbls := noteLabels(obj)
	for i := 0; i < len(kvs); i += 2 {
		key, want := kvs[i], kvs[i+1]
		got, ok := lbls[key].(string)
		if !ok || got != want {
			return false
		}
	}
	return true
}

//
// ────────────────────────────────────────────────────────────────
//   2. SPEC / STATUS HELPERS
// ────────────────────────────────────────────────────────────────
//

// noteSpec returns the spec map of a Kubernetes object.
// Safe for nil or missing spec.
//
//	{{ spec .children.cronjob }}
func noteSpec(obj interface{}) map[string]interface{} {
	if m, ok := obj.(map[string]interface{}); ok {
		if spec, ok := m["spec"].(map[string]interface{}); ok {
			return spec
		}
	}
	return map[string]interface{}{}
}

// noteStatus returns the status map of a Kubernetes object.
// Safe for nil or missing status.
//
//	{{ status .children.cronjob }}
func noteStatus(obj interface{}) map[string]interface{} {
	if m, ok := obj.(map[string]interface{}); ok {
		if status, ok := m["status"].(map[string]interface{}); ok {
			return status
		}
	}
	return map[string]interface{}{}
}

// noteGetStatus returns a single status field converted to its string
// representation. Returns "" when the field is absent, nil, or a structured
// value (map, slice) — use get or status for those instead.
//
//	{{ getStatus .children.deployment "readyReplicas" }} → "3"
func noteGetStatus(obj interface{}, key string) string {
	v := noteStatus(obj)[key]
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case map[string]interface{}, []interface{}:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

// noteHasStatus returns true when the given status field exists and is
// non-empty. Works for both scalar and structured (map, slice) fields.
//
//	{{ hasStatus .children.service "loadBalancer" }}
func noteHasStatus(obj interface{}, key string) bool {
	v, ok := noteStatus(obj)[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case string:
		return t != ""
	case map[string]interface{}:
		return len(t) > 0
	case []interface{}:
		return len(t) > 0
	default:
		return true
	}
}

//
// ────────────────────────────────────────────────────────────────
//   3. SAFE NESTED FIELD LOOKUP
// ────────────────────────────────────────────────────────────────
//

// noteGet retrieves a nested field from a Kubernetes object using a path.
// Returns nil if any segment is missing.
//
//	{{ get .children.cronjob "status" "lastScheduleTime" }}
func noteGet(obj interface{}, path ...string) interface{} {
	current := obj
	for _, p := range path {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[p]
		if current == nil {
			return nil
		}
	}
	return current
}

//
// ────────────────────────────────────────────────────────────────
//   4. OWNERREFERENCE HELPERS
// ────────────────────────────────────────────────────────────────
//

// noteOwnerKind returns the kind of the first ownerReference.
// Useful for debugging controller relationships.
//
//	{{ ownerKind .children.cronjob }}
func noteOwnerKind(obj interface{}) string {
	meta := noteMeta(obj)
	owners, ok := meta["ownerReferences"].([]interface{})
	if !ok || len(owners) == 0 {
		return ""
	}
	if first, ok := owners[0].(map[string]interface{}); ok {
		if kind, ok := first["kind"].(string); ok {
			return kind
		}
	}
	return ""
}

// noteOwnerName returns the name of the first ownerReference.
//
//	{{ ownerName .children.cronjob }}
func noteOwnerName(obj interface{}) string {
	meta := noteMeta(obj)
	owners, ok := meta["ownerReferences"].([]interface{})
	if !ok || len(owners) == 0 {
		return ""
	}
	if first, ok := owners[0].(map[string]interface{}); ok {
		if name, ok := first["name"].(string); ok {
			return name
		}
	}
	return ""
}

//
// ────────────────────────────────────────────────────────────────
//   5. CONDITION HELPERS
// ────────────────────────────────────────────────────────────────
//

// noteHasCondition checks if a Kubernetes status.conditions array
// contains a condition of a given type with status "True".
//
//	{{ hasCondition .children.deployment "Available" }}
func noteHasCondition(obj interface{}, condType string) bool {
	status := noteStatus(obj)
	conds, ok := status["conditions"].([]interface{})
	if !ok {
		return false
	}
	for _, c := range conds {
		if cm, ok := c.(map[string]interface{}); ok {
			if t, ok := cm["type"].(string); ok && t == condType {
				if s, ok := cm["status"].(string); ok && s == "True" {
					return true
				}
			}
		}
	}
	return false
}

// noteExists reports whether the given Kubernetes object exists in the
// template context. It returns true when obj is a non-nil map[string]interface{}.
//
//	{{ resourceExists .children.deployment }}
//	{{ resourceExists .children.secret }}
//	{{ resourceExists (get .children "configmap" "my-app") }}
func noteExists(obj interface{}) bool {
	if obj == nil {
		return false
	}
	m, ok := obj.(map[string]interface{})
	if !ok {
		return false
	}
	if v, _ := m["_placeholder"].(bool); v {
		return false
	}
	return true
}

//   "phase":              notePhase,
//   "conditionReason":    noteConditionReason,
//   "conditionMessage":   noteConditionMessage,
//   "isTerminating":      noteIsTerminating,
//   "generation":         noteGeneration,
//   "observedGeneration": noteObservedGeneration,
//   "isSynced":           noteIsSynced,

// ── Spec resource request accessors ──────────────────────────────────────────

// noteResourceCPU returns spec.resources.requests.cpu from any Kubernetes object.
// Safe at every level — returns "" when spec, resources, requests, or cpu is absent.
// Use in normalize blocks to default resource requests without nil pointer panics:
//
//	resources.requests.cpu: '{{ resourceCPU . | default "100m" }}'
func noteResourceCPU(obj interface{}) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	spec, _ := m["spec"].(map[string]interface{})
	resources, _ := spec["resources"].(map[string]interface{})
	requests, _ := resources["requests"].(map[string]interface{})
	v, _ := requests["cpu"].(string)
	return v
}

// noteResourceMemory returns spec.resources.requests.memory from any Kubernetes object.
// Safe at every level — returns "" when spec, resources, requests, or memory is absent.
//
//	resources.requests.memory: '{{ resourceMemory . | default "128Mi" }}'
func noteResourceMemory(obj interface{}) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	spec, _ := m["spec"].(map[string]interface{})
	resources, _ := spec["resources"].(map[string]interface{})
	requests, _ := resources["requests"].(map[string]interface{})
	v, _ := requests["memory"].(string)
	return v
}

// ── Phase ─────────────────────────────────────────────────────────────────────

// notePhase returns status.phase from a Kubernetes object.
// Returns "" when the field is absent or the object is nil.
// Safer than navigating .children.deployment.status.phase directly —
// no template error when status or phase is missing.
//
//	{{ if eq (phase .children.job) "Succeeded" }}
func notePhase(obj interface{}) string {
	status := noteStatus(obj)
	v, _ := status["phase"].(string)
	return v
}

// ── Condition detail ──────────────────────────────────────────────────────────

// noteConditionReason returns the reason field of a named condition.
// Returns "" when the condition is absent or has no reason.
//
//	{{ conditionReason .children.deployment "Available" }}
//	→ "MinimumReplicasAvailable" or ""
func noteConditionReason(obj interface{}, condType string) string {
	return conditionField(obj, condType, "reason")
}

// noteConditionMessage returns the message field of a named condition.
// Returns "" when the condition is absent or has no message.
//
//	{{ conditionMessage .children.deployment "Progressing" }}
//	→ "ReplicaSet has successfully progressed." or ""
func noteConditionMessage(obj interface{}, condType string) string {
	return conditionField(obj, condType, "message")
}

// conditionField is the shared implementation for noteConditionReason
// and noteConditionMessage.
func conditionField(obj interface{}, condType, field string) string {
	status := noteStatus(obj)
	conds, ok := status["conditions"].([]interface{})
	if !ok {
		return ""
	}
	for _, c := range conds {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := cm["type"].(string)
		if t != condType {
			continue
		}
		v, _ := cm[field].(string)
		return v
	}
	return ""
}

// ── Lifecycle ─────────────────────────────────────────────────────────────────

// noteIsTerminating returns true when the object has a deletionTimestamp set.
// An object with a deletionTimestamp is in the process of being deleted —
// it exists in the cluster but is not healthy to use as a dependency.
//
// Use in onDelete ordered sequences:
//
//	when:
//	  - field: "{{ isTerminating .children.job }}"
//	    equals: "false"
//
// Or in onReconcile to avoid routing traffic to terminating pods:
//
//	when:
//	  - field: "{{ isTerminating .children.deployment }}"
//	    equals: "false"
func noteIsTerminating(obj interface{}) bool {
	if obj == nil {
		return false
	}
	m, ok := obj.(map[string]interface{})
	if !ok {
		return false
	}
	meta, ok := m["metadata"].(map[string]interface{})
	if !ok {
		return false
	}
	// deletionTimestamp is set when the object is terminating.
	// It is either a non-empty string or a time.Time via JSON unmarshal.
	ts := meta["deletionTimestamp"]
	if ts == nil {
		return false
	}
	// String representation: non-empty means terminating
	if s, ok := ts.(string); ok {
		return s != "" && s != "null"
	}
	// Any other non-nil value is also terminating
	return true
}

// ── Generation tracking ───────────────────────────────────────────────────────

// noteGeneration returns metadata.generation as an int64.
// Returns 0 when the field is absent.
//
// Generation increments every time spec is changed.
//
//	{{ generation .children.deployment }}
//	→ 3
func noteGeneration(obj interface{}) int64 {
	if obj == nil {
		return 0
	}
	m, ok := obj.(map[string]interface{})
	if !ok {
		return 0
	}
	meta, ok := m["metadata"].(map[string]interface{})
	if !ok {
		return 0
	}
	return toInt64(meta["generation"])
}

// noteObservedGeneration returns status.observedGeneration as an int64.
// Returns 0 when the field is absent.
//
// observedGeneration is the generation the controller last acted on.
// When generation > observedGeneration, the controller has not yet
// processed the current spec change.
//
//	{{ observedGeneration .children.deployment }}
//	→ 3
func noteObservedGeneration(obj interface{}) int64 {
	status := noteStatus(obj)
	return toInt64(status["observedGeneration"])
}

// noteIsSynced returns true when metadata.generation equals
// status.observedGeneration, meaning the controller has fully processed
// the current spec.
//
// Use in when: conditions to wait for a child resource to stabilize
// before proceeding with dependent resources:
//
//	when:
//	  - field: "{{ isSynced .children.deployment }}"
//	    equals: "true"
//
// Or in status fields to surface whether children are current:
//
//   - path: deploymentSynced
//     value: "{{ isSynced .children.deployment }}"
func noteIsSynced(obj interface{}) bool {
	gen := noteGeneration(obj)
	obsGen := noteObservedGeneration(obj)
	// Both 0 means the resource has no generation tracking (statusless types).
	// Treat as synced to avoid blocking.
	if gen == 0 && obsGen == 0 {
		return true
	}
	return gen == obsGen
}

// toInt64 converts JSON number types to int64 safely.
// JSON unmarshals numbers as float64 by default.
func toInt64(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		var n int64
		fmt.Sscanf(t, "%d", &n)
		return n
	default:
		return 0
	}
}
