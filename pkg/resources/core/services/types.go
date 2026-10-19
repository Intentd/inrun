// pkg/resources/services/types.go
package services

// Spec is the fully resolved Service specification.
// Produced by resolving template expressions and merging static values.
// Passed directly to Create, Apply and Delete.
type Spec struct {
	// Name — Service name. Required.
	Name string

	// Namespace — target namespace. Required.
	Namespace string

	// Type — Kubernetes Service type: ClusterIP, NodePort, LoadBalancer.
	// Default: ClusterIP.
	Type string

	// Headless — when true, the Service is created without a clusterIP (clusterIP: None).
	// Used primarily for StatefulSets to enable stable network identities and per‑pod DNS:
	//   <podname>.<service>.<namespace>.svc.cluster.local
	// Set this to true when the Service is meant to back a StatefulSet or provide
	// direct pod‑to‑pod addressing rather than load‑balanced traffic.
	Headless bool

	// Protocol defines network protocols supported for things like container ports.
	// "TCP", "UDP", "SCTP"
	Protocol string

	// Port — Service port.
	Port int32

	// TargetPort — container port the Service routes to.
	TargetPort int32

	// Labels — applied to Service metadata.
	// Inrun always adds: managed-by=inrun, inrun-owner=<cr-name>
	Labels map[string]string

	// Selector —> service selector to route traffic to pods.
	Selector map[string]string
}
