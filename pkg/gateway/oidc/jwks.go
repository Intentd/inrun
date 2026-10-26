package oidc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
)

const (
	DefaultTTL = time.Hour

	// githubJWKSURL is fetched directly — GitHub does not require discovery.
	githubJWKSURL = "https://token.actions.githubusercontent.com/.well-known/jwks"
)

type cachedKeySet struct {
	keys      jose.JSONWebKeySet
	fetchedAt time.Time
}

// Cache fetches and caches JWKS by issuer URL.
// Safe for concurrent use.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*cachedKeySet
	ttl     time.Duration
	client  *http.Client
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*cachedKeySet),
		ttl:     ttl,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// keys returns the JWKS for the given issuer, fetching or refreshing as needed.
// discoveryBase is the base URL used to build the discovery URL — for most providers
// it equals issuer, but Vault uses {vaultURL}/v1/identity/oidc as the base.
func (c *Cache) keys(issuer, discoveryBase string) (*jose.JSONWebKeySet, error) {
	c.mu.RLock()
	entry, ok := c.entries[issuer]
	if ok && time.Since(entry.fetchedAt) < c.ttl {
		keys := entry.keys
		c.mu.RUnlock()
		return &keys, nil
	}
	c.mu.RUnlock()

	// Fetch outside the lock, then write.
	jwksURL, err := c.resolveJWKSURL(issuer, discoveryBase)
	if err != nil {
		return nil, fmt.Errorf("resolving JWKS URL for issuer %q: %w", issuer, err)
	}
	ks, err := c.fetchJWKS(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("fetching JWKS from %q: %w", jwksURL, err)
	}

	c.mu.Lock()
	c.entries[issuer] = &cachedKeySet{keys: *ks, fetchedAt: time.Now()}
	c.mu.Unlock()

	return ks, nil
}

// resolveJWKSURL returns the JWKS URL for the given issuer.
// GitHub is a special case — its JWKS URL is well-known and stable, so we skip discovery.
// All other issuers use OIDC discovery via discoveryBase (usually equal to issuer, but
// Vault uses a non-standard path: {url}/v1/identity/oidc).
func (c *Cache) resolveJWKSURL(issuer, discoveryBase string) (string, error) {
	if issuer == "https://token.actions.githubusercontent.com" {
		return githubJWKSURL, nil
	}
	return c.discoverJWKSURL(discoveryBase)
}

// discoverJWKSURL fetches the OIDC discovery document and extracts jwks_uri.
func (c *Cache) discoverJWKSURL(base string) (string, error) {
	discoveryURL := base + "/.well-known/openid-configuration"
	resp, err := c.client.Get(discoveryURL)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", discoveryURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: status %d", discoveryURL, resp.StatusCode)
	}
	var doc struct {
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("decoding discovery document: %w", err)
	}
	if doc.JWKSURI == "" {
		return "", fmt.Errorf("discovery document missing jwks_uri")
	}
	return doc.JWKSURI, nil
}

func (c *Cache) fetchJWKS(url string) (*jose.JSONWebKeySet, error) {
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	var ks jose.JSONWebKeySet
	if err := json.NewDecoder(resp.Body).Decode(&ks); err != nil {
		return nil, fmt.Errorf("decoding JWKS: %w", err)
	}
	return &ks, nil
}
