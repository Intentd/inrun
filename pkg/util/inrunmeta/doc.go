// Package inrunmeta holds every label, annotation and finalizer name Inrun
// writes (the inrun.dev domain), and Manager, which applies Inrun's standard
// labels to an object based on its Blueprint configuration. The mechanisms
// (finalizers, validation, JSON annotations, rotation) are
// github.com/intentd/tools/kube's.
package inrunmeta
