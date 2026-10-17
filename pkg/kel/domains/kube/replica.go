package note

import "text/template"

// replicaNotes registers helpers for inspecting ReplicaSet and Deployment-style
// replica status fields.
//
// Usage:
//
//	tmpl.Funcs(note.replicaNotes())
//
// Template examples:
//
//	"{{ allReplicasReady .children.deployment }}
//	{{ desiredReplicas .children.deployment }}
//	{{ readyReplicas .children.deployment }}
//	{{ availableReplicas .children.deployment }}
//	{{ updatedReplicas .children.deployment }}
//
// These helpers provide concise gates for rollout readiness, desired vs. actual
// replica counts, and updated pod availability—useful for orchestrating async
// workflows and ensuring dependent resources apply only after a rollout
// stabilizes.
func replicaNotes() template.FuncMap {
	return template.FuncMap{
		"allReplicasReady":  noteAllReplicasReady,
		"desiredReplicas":   noteDesiredReplicas,
		"readyReplicas":     noteReadyReplicas,
		"availableReplicas": noteAvailableReplicas,
		"updatedReplicas":   noteUpdatedReplicas,
		"rolloutComplete":   noteRolloutComplete,
	}
}

// ── Replica notes ─────────────────────────────────────────────────────────────

// noteAllReplicasReady returns true when readyReplicas equals the desired replica count.
// Handles scale-to-zero correctly: desired=0 and ready=0 → true.
//
//	{{ allReplicasReady .children.deployment }}
func noteAllReplicasReady(obj interface{}) bool {
	return noteReadyReplicas(obj) == noteDesiredReplicas(obj)
}

// noteRolloutComplete returns true when updatedReplicas equals desiredReplicas.
// This indicates that all pods are running the latest specification (the rollout
// has finished, though they may not all be ready yet).
//
//	{{ rolloutComplete .children.deployment }}  → true
func noteRolloutComplete(obj interface{}) bool {
	return noteUpdatedReplicas(obj) == noteDesiredReplicas(obj)
}

// noteDesiredReplicas returns spec.replicas.
// Returns 1 when the field is absent (Kubernetes default).
// Returns 0 when explicitly set to 0 (scale-to-zero).
//
//	{{ desiredReplicas .children.deployment }}  → 3
func noteDesiredReplicas(obj interface{}) int {
	spec := noteGet(obj, "spec")
	m, ok := spec.(map[string]interface{})
	if !ok {
		return 1
	}
	v, exists := m["replicas"]
	if !exists || v == nil {
		return 1
	}
	return int(toInt64(v))
}

// noteReadyReplicas returns status.readyReplicas safely. Returns 0 when not set.
//
//	{{ readyReplicas .children.deployment }}  → 2
func noteReadyReplicas(obj interface{}) int {
	status := noteStatus(obj)
	return int(toInt64(status["readyReplicas"]))
}

// noteAvailableReplicas returns status.availableReplicas safely.
//
//	{{ availableReplicas .children.deployment }}  → 2
func noteAvailableReplicas(obj interface{}) int {
	status := noteStatus(obj)
	return int(toInt64(status["availableReplicas"]))
}

// noteUpdatedReplicas returns status.updatedReplicas — pods running the current spec.
// When updatedReplicas == desiredReplicas, the rollout is complete.
//
//	{{ updatedReplicas .children.deployment }}  → 3
func noteUpdatedReplicas(obj interface{}) int {
	status := noteStatus(obj)
	return int(toInt64(status["updatedReplicas"]))
}
