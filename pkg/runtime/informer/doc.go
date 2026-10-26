// Package informer is the shared informer factory between the API server
// watch stream and the per-CRD work queues. It filters by namespace, computes
// sentinels on updates and enqueues keys; observe adds watches on secondary
// resources.
package informer
