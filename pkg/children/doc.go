// Package children reads the live state of the child resources a Blueprint
// declares for a custom resource. ReadChildren returns them keyed by kind and
// name, enriched with pods and warnings, for templates and status to use. It
// also resolves built-in kinds to their GVK and GVR and expands forEach
// declarations.
package children
