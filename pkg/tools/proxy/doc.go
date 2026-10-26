// Package proxy forwards the runtime, gateway and console to localhost.
// FindService discovers each component's Service by label, ResolveRuntimePod
// picks the current leader through its Lease, and RunAll keeps the
// port-forwards open, reconnecting when a pod restarts.
package proxy
