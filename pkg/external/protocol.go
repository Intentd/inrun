package external

import (
	"context"
	"sync"
	"time"

	"github.com/intentd/inrun/pkg/types"
)

// ProtocolClient executes one external call and returns the result map to
// inject under .external.<name>. The map is used directly by the runner.
//
// HTTP clients return: status, body, error, called + auto-parsed JSON keys.
// Non-HTTP clients return: result, raw, error, called + protocol-specific keys.
type ProtocolClient interface {
	Fetch(ctx context.Context, spec types.ExternalCallSpec, resolvedURL, resolvedQuery, resolvedBody, credential string) (map[string]interface{}, error)
}

func newProtocolClient(protocol types.ExternalProtocol) ProtocolClient {
	switch protocol {
	case "", types.ProtocolHTTP:
		return &httpProtocolClient{}
	case types.ProtocolPrometheus:
		return &prometheusClient{}
	case types.ProtocolRedis:
		return &redisClient{}
	case types.ProtocolPostgres:
		return &postgresClient{}
	case types.ProtocolMongo:
		return &mongoClient{}
	case types.ProtocolKafka:
		return &kafkaClient{}
	default:
		return &httpProtocolClient{} // unknown protocol falls through to HTTP; validate catches it
	}
}

// ── Cache ─────────────────────────────────────────────────────────────────────

type cachedResult struct {
	data      map[string]interface{}
	expiresAt time.Time
}

// resultCache holds per-(gvk, call-name, url, query) cached results.
// Key format: "<gvk>/<name>/<url>/<query>".
var resultCache sync.Map

func cacheKey(gvk, name, url, query string) string {
	return gvk + "\x00" + name + "\x00" + url + "\x00" + query
}

func cacheGet(key string) (map[string]interface{}, bool) {
	v, ok := resultCache.Load(key)
	if !ok {
		return nil, false
	}
	entry := v.(cachedResult)
	if time.Now().After(entry.expiresAt) {
		resultCache.Delete(key)
		return nil, false
	}
	return entry.data, true
}

func cacheSet(key string, data map[string]interface{}, ttl time.Duration) {
	resultCache.Store(key, cachedResult{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	})
}
