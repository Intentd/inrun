// Package housekeeper keeps the gateway's infrastructure in sync with the
// Blueprint for as long as the gateway runs: the TLS Secret, the webhook
// configurations, the operator namespace's labels, the conversion caBundle on
// CRDs and the Gateway API token Secrets. When one is deleted or drifts, the
// housekeeper puts it back.
//
// Watches are the fast path: deleting the TLS Secret or an Inrun webhook
// configuration, or modifying a conversion CRD, triggers a sync within one API
// round-trip. MODIFIED events on webhook configurations are ignored, because
// every sync that updates one fires one. A safety ticker (HOUSEKEEPER_SYNC_INTERVAL, default 30s)
// catches what watches miss. Triggers coalesce, so a burst of events causes one sync.
//
// Configurations are not removed when a pod restarts unless the Blueprint opts
// in with cleanupOnShutdown, so a new pod and an old one shutting down during a
// rollout do not undo each other's work.
package housekeeper
