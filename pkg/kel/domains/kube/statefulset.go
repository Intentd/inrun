package note

import "text/template"

// statefulSetNotes registers helpers for inspecting StatefulSet status fields
// and enriched PVC data embedded by the enrichment layer.
//
// Usage:
//
//	tmpl.Funcs(note.statefulSetNotes())
//
// Template examples:
//
//	{{ statefulSetPVCCount .children.statefulset }}
//	{{ statefulSetCurrentRevision .children.statefulset }}
//	{{ statefulSetUpdateRevision .children.statefulset }}
//
// statefulSetPVCCount requires enrich: [pvcs] on the CRD.
// statefulSetCurrentRevision and statefulSetUpdateRevision navigate the
// StatefulSet object directly — no enrichment required.
func statefulSetNotes() template.FuncMap {
	return template.FuncMap{
		"statefulSetCurrentRevision": noteStatefulSetCurrentRevision,
		"statefulSetUpdateRevision":  noteStatefulSetUpdateRevision,
		// Enriched PVC notes — require enrich: [pvcs] on the CRD.
		"statefulSetPVCCount": noteStatefulSetPVCCount,
	}
}

// ── StatefulSet notes (no enrichment required) ────────────────────────────────

// noteStatefulSetCurrentRevision returns status.currentRevision — the pod
// template hash of the currently running pods.
//
//	{{ statefulSetCurrentRevision .children.statefulset }}  → "my-sts-6d8f4b9c5"
func noteStatefulSetCurrentRevision(obj interface{}) string {
	status := noteStatus(obj)
	v, _ := status["currentRevision"].(string)
	return v
}

// noteStatefulSetUpdateRevision returns status.updateRevision — the pod
// template hash of the pending update. Equal to currentRevision when the
// rollout is complete.
//
//	{{ statefulSetUpdateRevision .children.statefulset }}  → "my-sts-6d8f4b9c5"
func noteStatefulSetUpdateRevision(obj interface{}) string {
	status := noteStatus(obj)
	v, _ := status["updateRevision"].(string)
	return v
}

// ── Enriched StatefulSet PVC notes (require enrich: [pvcs]) ──────────────────

// noteStatefulSetPVCCount returns the number of PVCs embedded under "_pvcs".
// Requires enrich: [pvcs] on the CRD.
//
//	{{ statefulSetPVCCount .children.statefulset }}  → 3
func noteStatefulSetPVCCount(obj interface{}) int {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return 0
	}
	pvcs, _ := m["_pvcs"].([]interface{})
	return len(pvcs)
}
