// Package presets holds Inrun's own presets: reconciler presets (workers,
// resync and queue depth for a CRD's reconciler) and autoscaler presets for
// the operator autoscaler. They tune Inrun itself, so they live here; the
// presets for resource fields are the library's, in pkg/resources/presets.
package presets
