// Package resources builds built-in kinds from resource intents. A resource
// intent is the flat form a remote reconciler can return for a Deployment,
// Service, Secret and so on: {"type": "deployment", "name": …, "image": …}
// instead of the full object. Each Build function decodes the fields for its
// kind, maps them onto the kind's spec, and builds the object with the kind's
// package in pkg/resources; Build picks the function by type.
package resources
