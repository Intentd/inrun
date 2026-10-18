// Package devserver provides a lightweight mock HTTP server for local development.
// Started via `inrun --dev-server`, it listens on :9999 and serves all endpoints
// used by the external: and full-stack example catalogs — no real services needed.
package devserver

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/intentd/inrun/pkg/logger"
)

const Port = 9999

// flagDefaults holds the out-of-box value for each known flag name.
// A flag not in this map defaults to "false".
var flagDefaults = map[string]bool{
	"v2Enabled":   true,
	"betaEnabled": false,
}

// flagState holds per-app per-flag overrides set via the toggle endpoint.
// key: "appName/flagName", value: current bool state.
var flagState sync.Map

// certState holds per-cert issued/pending overrides set via the toggle endpoint.
// key: cert name, value: true = issued, false = pending. Default is issued.
var certState sync.Map

// autoscaleFlipped tracks whether /autoscale-metrics is returning the
// overloaded payload. Toggled via POST /autoscale-metrics/flip.
var (
	autoscaleFlipped bool
	autoscaleMu      sync.Mutex
)

// workloadMetricsFlipped tracks whether /workload-metrics is returning the
// high-load payload. Toggled via POST /workload-metrics/flip.
var (
	workloadMetricsFlipped bool
	workloadMetricsMu      sync.Mutex
)

// jiraTransitions records POST /jira/transition calls, keyed by issueKey —
// readable back via GET /jira/issues/{key}. There's no real Jira here, so
// this in-memory store is the only way 03-jira-slack (serve example) can
// verify the call actually fired instead of just trusting a 200.
var jiraTransitions sync.Map

// slackMessages records POST /slack/notify calls, readable back via
// GET /slack/messages.
var (
	slackMessages   []slackMessage
	slackMessagesMu sync.Mutex
)

type slackMessage struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

// buckets backs /buckets/{name}, keyed by name.
var (
	buckets   = map[string]*bucket{}
	bucketsMu sync.Mutex
)

// volumes backs /volumes, keyed by the id the API assigns; volumeKeys maps
// an Idempotency-Key to the volume its first POST created.
var (
	volumes    = map[string]*volume{}
	volumeKeys = map[string]string{}
	volumeSeq  int
	volumesMu  sync.Mutex
	// volumeRequests records each request to /volumes, for GET /volumes/requests.
	volumeRequests []volumeRequest
)

type volumeRequest struct {
	Method         string `json:"method"`
	Path           string `json:"path"`
	IdempotencyKey string `json:"idempotencyKey,omitempty"`
	Status         int    `json:"status"`
}

type volume struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	SizeGB int    `json:"sizeGB"`
	Status string `json:"status"`
}

type bucket struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	QuotaGB  int    `json:"quotaGB"`
	Endpoint string `json:"endpoint"`
}

// certGet returns whether the cert is currently issued (true) or pending (false).
func certGet(name string) bool {
	if v, ok := certState.Load(name); ok {
		return v.(bool)
	}
	return true // default: issued
}

// certToggle flips the cert between issued and pending, returns the new state.
func certToggle(name string) bool {
	next := !certGet(name)
	certState.Store(name, next)
	return next
}

// flagGet returns the current bool value for a flag, preferring any toggle
// override over the hardcoded default.
func flagGet(app, flag string) bool {
	key := app + "/" + flag
	if v, ok := flagState.Load(key); ok {
		return v.(bool)
	}
	if def, ok := flagDefaults[flag]; ok {
		return def
	}
	return false
}

// flagToggle flips the current value for a flag and returns the new state.
func flagToggle(app, flag string) bool {
	next := !flagGet(app, flag)
	flagState.Store(app+"/"+flag, next)
	return next
}

// Start binds the dev server on the given port and returns immediately.
// The server runs in its own goroutine for the lifetime of the process.
func Start(port int) error {
	mux := http.NewServeMux()
	registerHandlers(mux)

	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("dev server: bind %s: %w", addr, err)
	}

	printDevBanner(port)

	go func() {
		if err := http.Serve(ln, mux); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("dev server error")
		}
	}()

	return nil
}

func printDevBanner(port int) {
	fmt.Printf("\n──────────────────────────────────────────\n")
	fmt.Printf("  Dev Server running on :%d\n", port)
	fmt.Printf("──────────────────────────────────────────\n")
	fmt.Printf("  GET  /health                    → 200 ok\n")
	fmt.Printf("  GET  /ready                     → 200 ready\n")
	fmt.Printf("  GET  /started                   → 200 started\n")
	fmt.Printf("  GET  /status/200                → 200 ok\n")
	fmt.Printf("  GET  /status/503                → 503 unavailable\n")
	fmt.Printf("  GET  /config/:name              → JSON config blob\n")
	fmt.Printf("  POST /sign                      → 200 signed (ignores token)\n")
	fmt.Printf("  POST /auth/token                → fake bearer token\n")
	fmt.Printf("  GET  /protected/:name           → 200 JSON (Bearer dev-token-abc123 required, else 401)\n")
	fmt.Printf("  GET  /resources/:name           → JSON resource stub\n")
	fmt.Printf("  GET  /flags/:name               → all flags JSON\n")
	fmt.Printf("  GET  /flags/:name/:flag         → single flag value (plain text)\n")
	fmt.Printf("  POST /flags/:name/:flag/toggle  → flip flag, returns new value\n")
	fmt.Printf("  GET  /sbom/:image               → SBOM report (nginx:vulnerable → high CVEs, nginx:unknown → 404)\n")
	fmt.Printf("  POST /cosign/verify             → 200 verified (nginx:unsigned → 403)\n")
	fmt.Printf("  GET  /vault/v1/secret/data/:path → Vault KV v2 (expired/missing in path → 403/404)\n")
	fmt.Printf("  POST /v1/data/:policy           → OPA decision (namespace=forbidden or name contains 'deny' → deny)\n")
	fmt.Printf("  GET  /certs/:name/status        → cert status: issued (200) or pending (202)\n")
	fmt.Printf("  POST /certs/:name/toggle        → flip cert between issued and pending\n")
	fmt.Printf("  GET  /autoscale-metrics         → payment system metrics (baseline: low queueDepth)\n")
	fmt.Printf("  POST /autoscale-metrics/flip    → toggle between baseline and overloaded payload\n")
	fmt.Printf("  GET  /workload-metrics          → worker pool metrics (baseline: low pendingJobs)\n")
	fmt.Printf("  POST /workload-metrics/flip     → toggle between baseline and high-load payload\n")
	fmt.Printf("  POST /jira/transition           → Jira transition stub (issueKey + transition → transitioned)\n")
	fmt.Printf("  GET  /jira/issues               → every transition recorded via /jira/transition\n")
	fmt.Printf("  GET  /jira/issues/:key          → transition last recorded for that key (404 if none)\n")
	fmt.Printf("  POST /slack/notify              → Slack notify stub (channel + text → ok)\n")
	fmt.Printf("  GET  /slack/messages            → every message recorded via /slack/notify\n")
	fmt.Printf("  PUT  /buckets/:name             → ensure a bucket (provisioning, then ready)\n")
	fmt.Printf("  GET  /buckets/:name             → the bucket, or 404\n")
	fmt.Printf("  DELETE /buckets/:name           → 202 deleting, then 404 once gone\n")
	fmt.Printf("  POST /volumes                   → create a volume; the API assigns the id (Idempotency-Key honoured)\n")
	fmt.Printf("  GET  /volumes                   → every volume, with a count\n")
	fmt.Printf("  GET  /volumes/requests          → every request the volume API received, in order\n")
	fmt.Printf("  GET  /volumes/:id               → the volume (provisioning, then ready), or 404\n")
	fmt.Printf("  DELETE /volumes/:id             → 202 deleting, then 404 once gone\n")
	fmt.Printf("──────────────────────────────────────────\n\n")
}
