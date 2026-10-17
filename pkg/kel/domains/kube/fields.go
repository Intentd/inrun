package note

import "text/template"

// fieldNotes registers direct accessors for common Kubernetes metadata fields.
//
// These are shorthand notes for fields that appear on virtually every resource:
// name, namespace, UID, resource version, and creation timestamp.
//
// Usage:
//
//	{{ resourceName .children.deployment }}      → "my-app"
//	{{ resourceNamespace .children.deployment }} → "production"
//	{{ resourceUID .children.deployment }}       → "4b3f8d21-..."
//	{{ resourceVersion .children.deployment }}   → "14872"
//	{{ creationTimestamp .children.deployment }} → "2024-01-15T10:30:00Z"
func fieldNotes() template.FuncMap {
	return template.FuncMap{
		"resourceName":      noteResourceName,
		"resourceNamespace": noteResourceNamespace,
		"resourceUID":       noteResourceUID,
		"resourceVersion":   noteResourceVersion,
		"creationTimestamp": noteCreationTimestamp,
	}
}

// noteResourceName returns metadata.name. Returns "" when absent.
//
//	{{ resourceName .children.deployment }} → "my-app"
func noteResourceName(obj interface{}) string {
	return metaStringField(obj, "name")
}

// noteResourceNamespace returns metadata.namespace. Returns "" for cluster-scoped resources.
//
//	{{ resourceNamespace .children.deployment }} → "production"
func noteResourceNamespace(obj interface{}) string {
	return metaStringField(obj, "namespace")
}

// noteResourceUID returns metadata.uid.
//
//	{{ resourceUID .children.deployment }} → "4b3f8d21-8e3a-4f8c-b9d2-1a2b3c4d5e6f"
func noteResourceUID(obj interface{}) string {
	return metaStringField(obj, "uid")
}

// noteResourceVersion returns metadata.resourceVersion — the Kubernetes etcd revision.
// Useful for detecting whether an object has been updated since last observation.
//
//	{{ resourceVersion .children.deployment }} → "14872"
func noteResourceVersion(obj interface{}) string {
	return metaStringField(obj, "resourceVersion")
}

// noteCreationTimestamp returns metadata.creationTimestamp as a string (RFC3339).
//
//	{{ creationTimestamp .children.deployment }} → "2024-01-15T10:30:00Z"
func noteCreationTimestamp(obj interface{}) string {
	return metaStringField(obj, "creationTimestamp")
}

// metaStringField extracts a string field from metadata.
func metaStringField(obj interface{}, field string) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	meta, ok := m["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	v, _ := meta[field].(string)
	return v
}
