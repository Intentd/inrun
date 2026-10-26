// pkg/logger/context.go
package logger

import (
	"context"
	"github.com/google/uuid"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	CRDKey       contextKey = "crd"
	ResourceKey  contextKey = "resource"
)

func WithRequestID(ctx context.Context) context.Context {
	return context.WithValue(ctx, RequestIDKey, uuid.New().String())
}

func WithCRD(ctx context.Context, crd string) context.Context {
	return context.WithValue(ctx, CRDKey, crd)
}

func WithResource(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, ResourceKey, key)
}

// Methods
func (c contextKey) String() string {
	return string(c)
}
