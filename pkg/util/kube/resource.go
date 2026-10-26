package kube

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// ResourceChecker caches resource existence checks with TTL.
// Safe for concurrent use.
type ResourceChecker struct {
	dynamic dynamic.Interface
	cache   map[string]cacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

type cacheEntry struct {
	exists    bool
	expiresAt time.Time
}

// NewResourceChecker creates a checker with default 30s TTL.
func NewResourceChecker(dynamic dynamic.Interface) *ResourceChecker {
	return &ResourceChecker{
		dynamic: dynamic,
		cache:   make(map[string]cacheEntry),
		ttl:     30 * time.Second,
	}
}

// NewResourceCheckerWithTTL creates a checker with custom TTL.
func NewResourceCheckerWithTTL(dynamic dynamic.Interface, ttl time.Duration) *ResourceChecker {
	return &ResourceChecker{
		dynamic: dynamic,
		cache:   make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

// Exists checks if a resource exists, using cached value if valid.
func (rc *ResourceChecker) Exists(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) bool {
	key := fmt.Sprintf("%s/%s/%s", gvr.String(), namespace, name)

	// Check cache with read lock
	rc.mu.RLock()
	if entry, ok := rc.cache[key]; ok && time.Now().Before(entry.expiresAt) {
		rc.mu.RUnlock()
		return entry.exists
	}
	rc.mu.RUnlock()

	// Cache miss or expired - check the API server
	exists := rc.resourceExists(ctx, gvr, namespace, name)

	// Store result in cache with write lock
	rc.mu.Lock()
	rc.cache[key] = cacheEntry{
		exists:    exists,
		expiresAt: time.Now().Add(rc.ttl),
	}
	rc.mu.Unlock()

	return exists
}

func (rc *ResourceChecker) resourceExists(ctx context.Context, gvr schema.GroupVersionResource, namespace, name string) bool {
	_, err := rc.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

// Invalidate removes a specific entry from the cache.
func (rc *ResourceChecker) Invalidate(gvr schema.GroupVersionResource, namespace, name string) {
	key := fmt.Sprintf("%s/%s/%s", gvr.String(), namespace, name)
	rc.mu.Lock()
	delete(rc.cache, key)
	rc.mu.Unlock()
}

// InvalidateAll clears the entire cache.
func (rc *ResourceChecker) InvalidateAll() {
	rc.mu.Lock()
	rc.cache = make(map[string]cacheEntry)
	rc.mu.Unlock()
}
