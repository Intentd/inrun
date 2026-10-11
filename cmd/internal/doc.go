// Package internal runs the runtime and gateway processes. Each Run function
// builds its process with an app package (pkg/runtime/app, pkg/gateway/app),
// starts it and waits for ctx to end; the app works out the rest.
//
// RunRuntime runs the runtime and leader election; only the leader
// reconciles. RunGateway runs the gateway: with TLS and the admission and
// conversion webhooks inside a pod, and the HTTP API alone outside one.
package internal
