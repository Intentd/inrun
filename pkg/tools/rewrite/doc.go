// package rewrite converts a controller-runtime reconciler for this runtime.
// Rewrite adapts the reconciler source, and Generate writes the files a
// project needs to build and test it: catalog.yaml, simulate.yaml, e2e.yaml,
// go.mod, Makefile and Dockerfile.
package rewrite
