package devserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/intentd/inrun/pkg/logger"
)

func registerHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/autoscale-metrics/flip", handle(autoscaleMetricsFlipHandler))
	mux.HandleFunc("/autoscale-metrics", handle(autoscaleMetricsHandler))
	mux.HandleFunc("/workload-metrics/flip", handle(workloadMetricsFlipHandler))
	mux.HandleFunc("/workload-metrics", handle(workloadMetricsHandler))
	mux.HandleFunc("/health", handle(healthHandler))
	mux.HandleFunc("/ready", handle(readyHandler))
	mux.HandleFunc("/started", handle(startedHandler))
	mux.HandleFunc("/status/", handle(statusHandler))
	mux.HandleFunc("/config/", handle(configHandler))
	mux.HandleFunc("/sign", handle(signHandler))
	mux.HandleFunc("/auth/token", handle(authTokenHandler))
	mux.HandleFunc("/protected/", handle(protectedHandler))
	mux.HandleFunc("/resources/", handle(resourcesHandler))
	mux.HandleFunc("/flags/", handle(flagsHandler))
	mux.HandleFunc("/sbom/", handle(sbomHandler))
	mux.HandleFunc("/cosign/verify", handle(cosignVerifyHandler))
	mux.HandleFunc("/vault/", handle(vaultHandler))
	mux.HandleFunc("/v1/data/", handle(opaPolicyHandler))
	mux.HandleFunc("/certs/", handle(certsHandler))
	mux.HandleFunc("/jira/transition", handle(jiraTransitionHandler))
	mux.HandleFunc("/jira/issues", handle(jiraIssueHandler))  // no key → list all
	mux.HandleFunc("/jira/issues/", handle(jiraIssueHandler)) // /jira/issues/{key} → single lookup
	mux.HandleFunc("/slack/notify", handle(slackNotifyHandler))
	mux.HandleFunc("/slack/messages", handle(slackMessagesHandler))
	mux.HandleFunc("/buckets/", handle(bucketHandler))
	mux.HandleFunc("/volumes", handle(recordVolume(volumesHandler)))
	mux.HandleFunc("/volumes/requests", handle(volumeRequestsHandler))
	mux.HandleFunc("/volumes/", handle(recordVolume(volumeHandler)))
}

// handle wraps a handler with debug logging.
func handle(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r)
		logger.Debug().Str("method", r.Method).Str("path", r.URL.Path).Msg("dev server request")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writePlain(w http.ResponseWriter, status int, s string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	fmt.Fprint(w, s) //nolint:errcheck
}

// GET /health → 200
func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /ready → 200
func readyHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// GET /started → 200
func startedHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// GET /status/200 → 200, GET /status/503 → 503
func statusHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.TrimPrefix(r.URL.Path, "/status/")
	switch code {
	case "200":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "503":
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown status code"})
	}
}

// GET /protected/:name → JSON blob; requires Authorization: Bearer dev-token-abc123.
// Returns 401 when the token is absent or wrong.
// Used by the external-auth fixture to verify auth.secretRef wires correctly.
func protectedHandler(w http.ResponseWriter, r *http.Request) {
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if strings.TrimSpace(bearer) != "dev-token-abc123" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/protected/")
	writeJSON(w, http.StatusOK, map[string]any{
		"name":       name,
		"env":        "production",
		"tokenValid": "true",
	})
}

// GET /config/:name → JSON config blob
func configHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/config/")
	writeJSON(w, http.StatusOK, map[string]any{
		"name":     name,
		"env":      "production",
		"debug":    "false",
		"replicas": 2,
	})
}

// POST /sign → 200 signed, or 403 if the image is nginx:not-secure.
func signHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Image string `json:"image"`
	}
	json.NewDecoder(r.Body).Decode(&payload) //nolint:errcheck

	if payload.Image == "nginx:not-secure" {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":  "image rejected by signing policy",
			"image":  payload.Image,
			"reason": "image is flagged as insecure — update spec.image to a trusted tag",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"signed": true})
}

// POST /auth/token → plain-string fake bearer token
func authTokenHandler(w http.ResponseWriter, r *http.Request) {
	writePlain(w, http.StatusOK, "dev-token-abc123")
}

// GET /resources/:name → JSON resource stub
func resourcesHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/resources/")
	writeJSON(w, http.StatusOK, map[string]any{
		"name":   name,
		"status": "active",
		"ready":  true,
	})
}

// GET /sbom/:image → SBOM report. nginx:vulnerable returns high CVE counts (operator
// gates the Deployment via a when: condition on the body). nginx:unknown → 404 (no SBOM).
func sbomHandler(w http.ResponseWriter, r *http.Request) {
	image := strings.TrimPrefix(r.URL.Path, "/sbom/")

	if image == "nginx:unknown" {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "no SBOM found for image",
			"image": image,
		})
		return
	}

	critical, high := 0, 0
	if image == "nginx:vulnerable" {
		critical, high = 3, 12
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"image":    image,
		"scanner":  "dev-scanner",
		"critical": critical,
		"high":     high,
		"medium":   2,
		"low":      7,
		"clean":    critical == 0 && high == 0,
	})
}

// POST /cosign/verify → 200 verified, or 403 if the image is nginx:unsigned.
// Chained after /sbom — the operator uses the SBOM result to gate this call.
func cosignVerifyHandler(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Image string `json:"image"`
	}
	json.NewDecoder(r.Body).Decode(&payload) //nolint:errcheck

	if payload.Image == "nginx:unsigned" {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error":  "no valid signature found",
			"image":  payload.Image,
			"reason": "image has no cosign signature — sign it before deploying",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"verified":  true,
		"image":     payload.Image,
		"signer":    "dev-signer@example.com",
		"algorithm": "ecdsa-p256",
	})
}

// GET /vault/v1/secret/data/:path → secret data, mimicking the Vault KV v2 API.
// Paths ending in /expired → 403 lease expired. Paths ending in /missing → 404.
func vaultHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/vault/v1/secret/data/")

	if strings.HasSuffix(path, "/expired") || strings.Contains(path, "expired") {
		writeJSON(w, http.StatusForbidden, map[string]any{
			"errors": []string{"permission denied — lease expired, secret must be rotated"},
		})
		return
	}

	if strings.HasSuffix(path, "/missing") || strings.Contains(path, "missing") {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"errors": []string{"secret not found at path: " + path},
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"data": map[string]string{
				"value":      "dev-secret-value",
				"expires_at": "2099-12-31T00:00:00Z",
			},
			"metadata": map[string]any{
				"version":       1,
				"created_time":  "2026-01-01T00:00:00Z",
				"deletion_time": "",
				"destroyed":     false,
			},
		},
	})
}

// POST /v1/data/:policy → OPA decision. Mimics the OPA REST API wire format.
// Denies when input.namespace == "forbidden" or input.name contains "deny".
func opaPolicyHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Input map[string]any `json:"input"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

	deny := false
	reason := ""

	ns, _ := body.Input["namespace"].(string)
	name, _ := body.Input["name"].(string)

	if ns == "forbidden" {
		deny = true
		reason = "namespace 'forbidden' is not permitted by org policy"
	} else if strings.Contains(name, "deny") {
		deny = true
		reason = "resource name contains a denied prefix — rename the CR"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"result": map[string]any{
			"allow":  !deny,
			"deny":   deny,
			"reason": reason,
		},
	})
}

// certsHandler routes /certs/* requests:
//
//	GET  /certs/:name/status → cert status (issued/pending)
//	POST /certs/:name/toggle → flip between issued and pending, return new status
func certsHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/certs/")
	rest = strings.TrimSuffix(rest, "/")
	parts := strings.SplitN(rest, "/", 2)

	if len(parts) < 2 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "use /certs/:name/status or /certs/:name/toggle"})
		return
	}

	name, action := parts[0], parts[1]

	switch {
	case action == "status" && r.Method == http.MethodGet:
		issued := certGet(name)
		status, code := "issued", http.StatusOK
		if !issued {
			status, code = "pending", http.StatusAccepted
		}
		writeJSON(w, code, map[string]any{
			"name":       name,
			"status":     status,
			"issuer":     "dev-ca",
			"expires_at": "2099-12-31T00:00:00Z",
		})

	case action == "toggle" && r.Method == http.MethodPost:
		next := certToggle(name)
		status := "issued"
		if !next {
			status = "pending"
		}
		writePlain(w, http.StatusOK, status)

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown certs route"})
	}
}

// POST /jira/transition → 200 Jira transition response.
// Mimics the Jira REST API POST /rest/api/2/issue/{key}/transitions wire format.
// Stores the transition so GET /jira/issues/{key} can read it back.
func jiraTransitionHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IssueKey   string `json:"issueKey"`
		Transition string `json:"transition"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
	jiraTransitions.Store(body.IssueKey, body.Transition)
	writeJSON(w, http.StatusOK, map[string]any{
		"issueKey":   body.IssueKey,
		"transition": body.Transition,
		"status":     "transitioned",
	})
}

// GET /jira/issues → every transition recorded via POST /jira/transition so far.
// GET /jira/issues/{key} → the transition last recorded for that issue key,
// or 404 if /jira/transition was never called for it.
func jiraIssueHandler(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/jira/issues")
	key = strings.Trim(key, "/")

	if key == "" {
		var issues []map[string]any
		jiraTransitions.Range(func(k, v any) bool {
			issues = append(issues, map[string]any{"issueKey": k, "transition": v})
			return true
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"count":  len(issues),
			"issues": issues,
		})
		return
	}

	transition, ok := jiraTransitions.Load(key)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no transition recorded for issue " + key})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"issueKey":   key,
		"transition": transition,
	})
}

// POST /slack/notify → 200 Slack acknowledgement.
// Accepts the Slack incoming-webhook wire format (channel + text).
// Appends the message so GET /slack/messages can read it back.
func slackNotifyHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Channel string `json:"channel"`
		Text    string `json:"text"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck

	slackMessagesMu.Lock()
	slackMessages = append(slackMessages, slackMessage{Channel: body.Channel, Text: body.Text})
	slackMessagesMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"channel": body.Channel,
	})
}

// GET /slack/messages → every message recorded via POST /slack/notify so far.
func slackMessagesHandler(w http.ResponseWriter, r *http.Request) {
	slackMessagesMu.Lock()
	messages := append([]slackMessage(nil), slackMessages...)
	slackMessagesMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"count":    len(messages),
		"messages": messages,
	})
}

// flagsHandler routes /flags/* requests:
//
//	GET  /flags/:name              → all flags as JSON
//	GET  /flags/:name/:flag        → single flag value as plain text ("true"/"false")
//	POST /flags/:name/:flag/toggle → flip the flag, return new value as plain text
func flagsHandler(w http.ResponseWriter, r *http.Request) {
	// Strip the /flags/ prefix and split remaining path segments.
	rest := strings.TrimPrefix(r.URL.Path, "/flags/")
	rest = strings.TrimSuffix(rest, "/")
	parts := strings.SplitN(rest, "/", 3)

	switch {
	// GET /flags/:name
	case len(parts) == 1:
		name := parts[0]
		writeJSON(w, http.StatusOK, map[string]any{
			"name":        name,
			"v2Enabled":   flagGet(name, "v2Enabled"),
			"betaEnabled": flagGet(name, "betaEnabled"),
		})

	// GET /flags/:name/:flag
	case len(parts) == 2 && r.Method == http.MethodGet:
		name, flag := parts[0], parts[1]
		writePlain(w, http.StatusOK, fmt.Sprintf("%v", flagGet(name, flag)))

	// POST /flags/:name/:flag/toggle
	case len(parts) == 3 && parts[2] == "toggle" && r.Method == http.MethodPost:
		name, flag := parts[0], parts[1]
		next := flagToggle(name, flag)
		writePlain(w, http.StatusOK, fmt.Sprintf("%v", next))

	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown flags route"})
	}
}

// autoscaleMetricsHandler serves GET /autoscale-metrics.
// Returns a baseline (low load) or overloaded (high load) metrics payload
// depending on the current flip state. The payload shape mirrors the Inrun
// /blueprint/{crd} metrics response so that cross.<crd>.metrics.* conditions
// can resolve against it directly.
func autoscaleMetricsHandler(w http.ResponseWriter, _ *http.Request) {
	autoscaleMu.Lock()
	flipped := autoscaleFlipped
	autoscaleMu.Unlock()

	var m map[string]interface{}
	if flipped {
		m = map[string]interface{}{
			"errorRatePercent":       0,
			"queueDepth":             98,
			"reconcileDurationP95Ms": 1244.18,
			"workersBusyPercent":     100,
			"workersIdlePercent":     0,
		}
	} else {
		m = map[string]interface{}{
			"errorRatePercent":       0,
			"queueDepth":             12,
			"reconcileDurationP95Ms": 142.3,
			"workersBusyPercent":     18,
			"workersIdlePercent":     82,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"metrics": m})
}

// autoscaleMetricsFlipHandler serves POST /autoscale-metrics/flip.
// Toggles between baseline and overloaded payload; returns the new state.
func autoscaleMetricsFlipHandler(w http.ResponseWriter, _ *http.Request) {
	autoscaleMu.Lock()
	autoscaleFlipped = !autoscaleFlipped
	flipped := autoscaleFlipped
	autoscaleMu.Unlock()

	state := "baseline"
	if flipped {
		state = "overloaded"
	}
	writePlain(w, http.StatusOK, state)
}

// workloadMetricsHandler serves GET /workload-metrics.
// Returns a baseline (low load) or high-load worker pool metrics payload
// depending on the current flip state. Used by workload autoscaler examples.
func workloadMetricsHandler(w http.ResponseWriter, _ *http.Request) {
	workloadMetricsMu.Lock()
	flipped := workloadMetricsFlipped
	workloadMetricsMu.Unlock()

	var m map[string]interface{}
	if flipped {
		m = map[string]interface{}{
			"pendingJobs":    152,
			"processingRate": 38,
			"errorRate":      2,
			"workerUtilPct":  94,
		}
	} else {
		m = map[string]interface{}{
			"pendingJobs":    8,
			"processingRate": 120,
			"errorRate":      0,
			"workerUtilPct":  22,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"queue": m})
}

// workloadMetricsFlipHandler serves POST /workload-metrics/flip.
// Toggles between baseline and high-load payload; returns the new state.
func workloadMetricsFlipHandler(w http.ResponseWriter, _ *http.Request) {
	workloadMetricsMu.Lock()
	workloadMetricsFlipped = !workloadMetricsFlipped
	flipped := workloadMetricsFlipped
	workloadMetricsMu.Unlock()

	state := "baseline"
	if flipped {
		state = "high-load"
	}
	writePlain(w, http.StatusOK, state)
}

// /buckets/{name} is a storage API that knows nothing about Inrun, for the
// existing-service example (a mapped remote reconciler):
//
//	PUT    → ensure: 200, provisioning on the first call and ready after
//	GET    → 200 with the bucket, 404 when there is none
//	DELETE → 202 deleting on the first call, 404 once it is gone
func bucketHandler(w http.ResponseWriter, r *http.Request) {
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/buckets"), "/")
	if name == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bucket name required"})
		return
	}

	bucketsMu.Lock()
	defer bucketsMu.Unlock()
	b, exists := buckets[name]

	switch r.Method {
	case http.MethodPut:
		var body struct {
			QuotaGB int `json:"quotaGB"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		if !exists {
			b = &bucket{ID: "b-" + name, Status: "provisioning", Endpoint: "https://b-" + name + ".storage.example"}
			buckets[name] = b
		} else if b.Status == "provisioning" {
			b.Status = "ready"
		}
		b.QuotaGB = body.QuotaGB
		writeJSON(w, http.StatusOK, b)

	case http.MethodGet:
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no bucket " + name})
			return
		}
		writeJSON(w, http.StatusOK, b)

	case http.MethodDelete:
		switch {
		case !exists:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no bucket " + name})
		case b.Status == "deleting":
			delete(buckets, name)
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no bucket " + name})
		default:
			b.Status = "deleting"
			writeJSON(w, http.StatusAccepted, b)
		}

	default:
		writePlain(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// /volumes is a storage API that assigns the id itself: POST creates a
// volume and answers its id, and later calls address it by that id. A POST
// carrying an Idempotency-Key it has seen answers the first volume instead
// of creating another, the way Stripe-style APIs do; without a key, every
// POST creates a new volume. GET /volumes lists them all, so a test can see
// whether a create ran twice.
func volumesHandler(w http.ResponseWriter, r *http.Request) {
	volumesMu.Lock()
	defer volumesMu.Unlock()

	switch r.Method {
	case http.MethodPost:
		key := r.Header.Get("Idempotency-Key")
		if id, seen := volumeKeys[key]; key != "" && seen {
			if v, ok := volumes[id]; ok {
				writeJSON(w, http.StatusCreated, v)
				return
			}
		}
		var body struct {
			Name   string `json:"name"`
			SizeGB int    `json:"sizeGB"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		volumeSeq++
		v := &volume{ID: fmt.Sprintf("v-%d", volumeSeq), Name: body.Name, SizeGB: body.SizeGB, Status: "provisioning"}
		volumes[v.ID] = v
		if key != "" {
			volumeKeys[key] = v.ID
		}
		writeJSON(w, http.StatusCreated, v)

	case http.MethodGet:
		items := make([]*volume, 0, len(volumes))
		for _, v := range volumes {
			items = append(items, v)
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		writeJSON(w, http.StatusOK, map[string]interface{}{"count": len(items), "items": items})

	default:
		writePlain(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// volumeHandler serves /volumes/{id}: GET answers the volume (provisioning,
// then ready) or 404; DELETE answers 202 deleting, then 404 once it is gone.
func volumeHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/volumes"), "/")
	volumesMu.Lock()
	defer volumesMu.Unlock()
	v, exists := volumes[id]

	switch r.Method {
	case http.MethodGet:
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no volume " + id})
			return
		}
		writeJSON(w, http.StatusOK, v)
		if v.Status == "provisioning" {
			v.Status = "ready" // the next read sees it ready
		}

	case http.MethodDelete:
		switch {
		case !exists:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no volume " + id})
		case v.Status == "deleting":
			delete(volumes, id)
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no volume " + id})
		default:
			v.Status = "deleting"
			writeJSON(w, http.StatusAccepted, v)
		}

	default:
		writePlain(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// recordVolume records each request to the volume API and the status it got.
func recordVolume(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h(rec, r)
		volumesMu.Lock()
		volumeRequests = append(volumeRequests, volumeRequest{
			Method: r.Method, Path: r.URL.Path, IdempotencyKey: r.Header.Get("Idempotency-Key"), Status: rec.status,
		})
		volumesMu.Unlock()
	}
}

// volumeRequestsHandler answers every request the volume API has received.
func volumeRequestsHandler(w http.ResponseWriter, r *http.Request) {
	volumesMu.Lock()
	defer volumesMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]interface{}{"count": len(volumeRequests), "items": volumeRequests})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
