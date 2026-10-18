// Package runtimeapi is the runtime's HTTP API as other components use it:
// the routes (paths.go), the response types, the query client the gateway
// uses to read live state (NewClient), and the cross-CR fetch another Inrun
// instance's CR detail is read with.
//
// The runtime and its clients both import this package, so it must stay
// small: it imports no runtime or gateway package itself.
package runtimeapi
