package template

import (
	"context"

	"github.com/intentd/inrun/pkg/utils"
)

type inrunContextKey struct{}

// ContextWithInrunContext stores an InrunContext in a context.Context so that
// NewResolver can pick it up automatically without any call-site changes.
func ContextWithInrunContext(ctx context.Context, inrun InrunContext) context.Context {
	return context.WithValue(ctx, inrunContextKey{}, inrun)
}

// inrunContextFromCtx extracts the InrunContext from ctx, returning a zero value
// when none has been stored.
func inrunContextFromCtx(ctx context.Context) InrunContext {
	if v, ok := ctx.Value(inrunContextKey{}).(InrunContext); ok {
		return v
	}
	return InrunContext{}
}

// InrunContext holds runtime facts about the Inrun process — constant for the
// lifetime of the process. Injected into every resolver as ".inrun.*" so any
// template expression can read them without a note or a function call.
//
//	{{ .inrun.inPod }}   → true when running inside a Kubernetes pod
//	{{ .inrun.namespace }}   → the namespace Inrun is deployed in
//	{{ .inrun.version }}     → the running Inrun version string
type InrunContext struct {
	InPod     bool   `json:"inPod"`
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
}

// NewInrunContext builds the runtime context. Called once at startup.
func NewInrunContext(namespace, version string) InrunContext {
	return InrunContext{
		InPod:     utils.IsRunningInPod(),
		Namespace: namespace,
		Version:   version,
	}
}

// asMap returns the context as a plain map for injection into the resolver data map.
func (o InrunContext) asMap() map[string]interface{} {
	return map[string]interface{}{
		"inPod":     o.InPod,
		"namespace": o.Namespace,
		"version":   o.Version,
	}
}
