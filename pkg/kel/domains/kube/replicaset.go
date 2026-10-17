package note

import "text/template"

// replicaSetNotes registers helpers for inspecting ReplicaSet owner relationships
// and Deployment replica set history.
//
// Usage:
//
//	tmpl.Funcs(note.replicaSetNotes())
//
// Template examples:
//
//	{{ replicaSetOwnerName .children.replicaset }}
//	{{ replicaSetOwnerKind .children.replicaset }}
//	{{ deploymentReplicaSetCount .children.deployment }}
//	{{ deploymentReplicaSets .children.deployment }}
//	{{ oldDeploymentReplicaSets .children.deployment }}
//
// replicaSetOwnerName and replicaSetOwnerKind require enrich: [owner] on the CRD.
// deploymentReplicaSetCount requires enrich: [replicasets] on the CRD.
func replicaSetNotes() template.FuncMap {
	return template.FuncMap{
		// Enriched owner notes — require enrich: [owner] on the CRD.
		"replicaSetOwnerName": noteReplicaSetOwnerName,
		"replicaSetOwnerKind": noteReplicaSetOwnerKind,
		// Enriched deployment notes — require enrich: [replicasets] on the CRD.
		"deploymentReplicaSetCount": noteDeploymentReplicaSetCount,
		"deploymentReplicaSets":     noteDeploymentReplicaSetNames,
		"oldDeploymentReplicaSets":  noteDeploymentOldReplicaSetNames,
	}
}

// ── ReplicaSet owner notes (require enrich: [owner]) ─────────────────────────

// noteReplicaSetOwnerName returns the name of the controller that owns this
// ReplicaSet, from _owner.name. Typically a Deployment name.
// Requires enrich: [owner] on the CRD.
//
//	{{ replicaSetOwnerName .children.replicaset }}  → "my-app"
func noteReplicaSetOwnerName(obj interface{}) string {
	owner := getOwner(obj)
	if owner == nil {
		return ""
	}
	v, _ := owner["name"].(string)
	return v
}

// noteReplicaSetOwnerKind returns the kind of the controller that owns this
// ReplicaSet, from _owner.kind. Typically "Deployment".
// Requires enrich: [owner] on the CRD.
//
//	{{ replicaSetOwnerKind .children.replicaset }}  → "Deployment"
func noteReplicaSetOwnerKind(obj interface{}) string {
	owner := getOwner(obj)
	if owner == nil {
		return ""
	}
	v, _ := owner["kind"].(string)
	return v
}

func getOwner(obj interface{}) map[string]interface{} {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}
	o, _ := m["_owner"].(map[string]interface{})
	return o
}

// ── Deployment ReplicaSet notes (require enrich: [replicasets]) ───────────────

// noteDeploymentReplicaSetCount returns the number of ReplicaSets owned by the
// Deployment. Includes inactive (scaled-to-zero) ReplicaSets kept for rollback.
// Requires enrich: [replicasets] on the CRD.
//
//	{{ deploymentReplicaSetCount .children.deployment }}  → 2
func noteDeploymentReplicaSetCount(obj interface{}) int {
	rsList := noteDeploymentReplicaSets(obj)
	return len(rsList)
}

// noteDeploymentReplicaSetNames returns the names of ReplicaSets owned by the
// Deployment. Includes inactive (scaled-to-zero) ReplicaSets kept for rollback.
// Requires enrich: [replicasets] on the CRD.
//
//	{{ deploymentReplicaSets .children.deployment }} → "rs-a", "rs-b"
func noteDeploymentReplicaSetNames(obj interface{}) string {
	rsList := noteDeploymentReplicaSets(obj)
	if rsList == nil {
		return ""
	}

	out := make([]string, 0, len(rsList))
	for _, raw := range rsList {
		rs, _ := raw.(map[string]interface{})
		meta, _ := rs["metadata"].(map[string]interface{})
		name, _ := meta["name"].(string)
		if name != "" {
			out = append(out, name)
		}
	}

	return join(out, ", ")
}

// noteDeploymentOldReplicaSetNames returns the names of old ReplicaSets owned
// by the Deployment. Requires enrich: [replicasets].
// Requires enrich: [replicasets] on the CRD.
//
//	{{ oldDeploymentReplicaSets .children.deployment }}  → "rs-a", "rs-b"
func noteDeploymentOldReplicaSetNames(obj interface{}) string {
	old := noteDeploymentOldReplicaSets(obj)
	if old == nil {
		return ""
	}

	out := make([]string, 0, len(old))
	for _, raw := range old {
		rs, _ := raw.(map[string]interface{})
		meta, _ := rs["metadata"].(map[string]interface{})
		name, _ := meta["name"].(string)
		if name != "" {
			out = append(out, name)
		}
	}

	return join(out, ", ")
}

// noteDeploymentReplicaSets returns the list of ReplicaSets owned by the
// Deployment. Includes inactive (scaled-to-zero) ReplicaSets kept for rollback.
func noteDeploymentReplicaSets(obj interface{}) []interface{} {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}
	rs, _ := m["_replicaSets"].([]interface{})
	return rs
}

// noteDeploymentOldReplicaSets returns the ReplicaSets owned by the Deployment
// that are considered "old" – i.e., any ReplicaSet whose name is NOT the
// deployment's status.currentReplicaSet.
func noteDeploymentOldReplicaSets(obj interface{}) []interface{} {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}

	// Get the deployment's status.currentReplicaSet name
	status, _ := m["status"].(map[string]interface{})
	currentRSName, _ := status["currentReplicaSet"].(string)
	if currentRSName == "" {
		// If status is not yet populated,  wait for status to appear.
		return nil
	}

	rsList, _ := m["_replicaSets"].([]interface{})
	if rsList == nil {
		return nil
	}

	out := []interface{}{}
	for _, raw := range rsList {
		rs, _ := raw.(map[string]interface{})
		meta, _ := rs["metadata"].(map[string]interface{})
		name, _ := meta["name"].(string)

		if name != "" && name != currentRSName {
			out = append(out, rs)
		}
	}
	return out
}
