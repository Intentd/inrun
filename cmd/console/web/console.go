package web

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ccversion "github.com/intentd/inrun/console/version"
)

//go:embed assets/templates/*.html assets/static/* assets/static/css/* assets/static/js/*
var Assets embed.FS

const TemplateDir = "assets/templates"

// ─────────────────────────────────────────────────────────────────────────────
// Config
// ─────────────────────────────────────────────────────────────────────────────

type Config struct {
	RefreshInterval      time.Duration
	LogLevel             string
	Version              string
	EnableRuntimeManager bool
	NoLogin              bool
	GatewayToken         string
}

// ─────────────────────────────────────────────────────────────────────────────
// Instance — one connected Inrun runtime
// ─────────────────────────────────────────────────────────────────────────────
type Instance struct {
	URL             string
	Client          *Client
	Catalog         *CatalogResponse
	Healthy         bool
	LastError       string
	LastCheck       time.Time
	Status          string // "online", "starting" or "degraded"
	GatewayEndpoint string // from runtime /catalog; empty when no gateway
}

// ─────────────────────────────────────────────────────────────────────────────
// Console
// ─────────────────────────────────────────────────────────────────────────────

type Console struct {
	urls         []string
	instances    map[string]*Instance // keyed by URL
	mu           sync.RWMutex
	config       Config
	ready        atomic.Bool
	subscribers  sync.Map // map[chan struct{}]struct{}
	gatewayToken string
	done         chan struct{} // closed by Close; stops fetching and SSE streams
	closeOnce    sync.Once
}

// New creates and starts a Console. Background fetches begin immediately.
func New(urls []string, config Config) *Console {
	// Load previously stored URLs
	stored, _ := LoadRuntimeStorage()
	allURLs := make(map[string]bool)
	for _, u := range urls {
		allURLs[normalizeURL(u)] = true
	}
	if stored != nil {
		for _, u := range stored.URLs {
			allURLs[normalizeURL(u)] = true
		}
	}
	finalURLs := make([]string, 0, len(allURLs))
	for u := range allURLs {
		finalURLs = append(finalURLs, u)
	}

	instances := make(map[string]*Instance)
	for _, url := range finalURLs {
		instances[url] = &Instance{
			URL:    url,
			Client: NewClient(url, config.RefreshInterval, config.LogLevel),
		}
	}
	c := &Console{
		urls:         finalURLs,
		instances:    instances,
		config:       config,
		gatewayToken: config.GatewayToken,
		done:         make(chan struct{}),
	}
	go c.backgroundFetchLoop()
	return c
}

// Close stops background fetches and ends open SSE streams, so an HTTP
// server shutdown is not held open by them. Safe to call more than once.
func (c *Console) Close() { c.closeOnce.Do(func() { close(c.done) }) }

func (c *Console) IsReady() bool { return c.ready.Load() }
func (c *Console) NoLogin() bool { return c.config.NoLogin }

// ─────────────────────────────────────────────────────────────────────────────
// Background fetch
// ─────────────────────────────────────────────────────────────────────────────

func (c *Console) backgroundFetchLoop() {
	c.fetchAllCatalogs()
	t := time.NewTicker(c.config.RefreshInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.fetchAllCatalogs()
		case <-c.done:
			return
		}
	}
}

func (c *Console) fetchAllCatalogs() {
	// Snapshot instance URLs without holding the lock during network I/O.
	// Holding a write lock across HTTP calls blocks all concurrent reads
	// (CRD detail pages, snapshot API) for the full fetch duration, causing
	// intermittent "pending / 0 reconciles" displays even when the runtime is healthy.
	c.mu.RLock()
	urls := make([]string, 0, len(c.instances))
	for u := range c.instances {
		urls = append(urls, u)
	}
	c.mu.RUnlock()

	anyOK := false

	for _, u := range urls {
		c.mu.RLock()
		inst, ok := c.instances[u]
		c.mu.RUnlock()
		if !ok {
			continue // instance was removed between snapshot and now
		}

		now := time.Now()
		cat, err := inst.Client.FetchCatalog()

		// Update only this instance under a brief write lock
		c.mu.Lock()
		if inst, ok = c.instances[u]; ok {
			inst.LastCheck = now
			if err != nil {
				inst.Catalog = nil
				inst.Status = "offline"
				inst.Healthy = false
				inst.LastError = err.Error()
				Warnf("fetch catalog from %s: %v", u, err)
			} else {
				inst.GatewayEndpoint = cat.GatewayEndpoint
				if cat.IsLeader {
					// Leader pod — data is authoritative, always update.
					inst.Catalog = cat
					inst.Status = "online"
					inst.Healthy = true
					inst.LastError = ""
				} else if inst.Catalog == nil {
					// Follower pod but we have nothing yet — accept it so the
					// catalog appears immediately rather than waiting for a lucky
					// tick that hits the leader. It may show CRDs as pending until
					// the next leader response overwrites it.
					inst.Catalog = cat
					inst.Status = "starting"
					inst.Healthy = false
					inst.LastError = ""
				}
				// Follower + already have good data → keep last known-good snapshot,
				// discard this response so the display does not flip.
				Debugf("fetched catalog %q from %s (%d CRDs, gateway=%q)",
					cat.Name, u, len(cat.CRDs), cat.GatewayEndpoint)
			}
		}
		c.mu.Unlock()

		if err == nil {
			anyOK = true
		}
	}

	c.ready.Store(anyOK)
	c.notifySubscribers()
}

// notifySubscribers sends a signal to all connected SSE clients.
func (c *Console) notifySubscribers() {
	c.subscribers.Range(func(key, _ interface{}) bool {
		ch := key.(chan struct{})
		select {
		case ch <- struct{}{}:
		default:
		}
		return true
	})
}

// ServeSSE handles Server-Sent Events for live page reloads.
func (c *Console) ServeSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan struct{}, 1)
	c.subscribers.Store(ch, struct{}{})
	defer c.subscribers.Delete(ch)

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Send initial connection confirmation
	fmt.Fprintf(w, "data: connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-ch:
			fmt.Fprintf(w, "data: update\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-c.done:
			return
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Instance lookups
// ─────────────────────────────────────────────────────────────────────────────

// instanceByCatalogName returns the instance whose loaded catalog has the given name.
func (c *Console) instanceByCatalogName(name string) (*Instance, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, inst := range c.instances {
		if inst.Catalog != nil && inst.Catalog.Name == name {
			return inst, true
		}
	}
	return nil, false
}

// clientFor returns the Client for an instance URL.
// Falls back to the first available client if the URL is not found.
func (c *Console) clientFor(instanceURL string) *Client {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if inst, ok := c.instances[instanceURL]; ok {
		return inst.Client
	}
	// URL not found — return first available client as fallback
	for _, inst := range c.instances {
		return inst.Client
	}
	return NewClient(instanceURL, c.config.RefreshInterval, c.config.LogLevel)
}

// ─────────────────────────────────────────────────────────────────────────────
// Router
// ─────────────────────────────────────────────────────────────────────────────

// ServeHTTP dispatches all /console/** requests.
//
// Route table (after stripping /console prefix):
//
//	/                                                   → index
//	/assets/**                                          → static files
//	/metrics                                            → metrics page
//	/debug/file                                         → debug
//	/docs                                               → docs landing
//	/docs/{catalog}/{crd}                               → CRD docs page
//	/catalog/{catalog}                                  → catalog panel
//	/catalog/{catalog}/crd/{crd}                        → CRD detail
//	/catalog/{catalog}/crd/{crd}/cr                     → CR list
//	/catalog/{catalog}/crd/{crd}/cr/{name}              → CR detail (cluster-scoped)
//	/catalog/{catalog}/crd/{crd}/cr/{ns}/{name}         → CR detail (namespaced)
func (c *Console) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/console")
	if path == "" || path == "/" {
		c.handleIndex(w, r)
		return
	}

	Debugf("%s %s", r.Method, path)

	switch {
	case strings.HasPrefix(path, "/assets/"):
		c.serveAsset(w, r, strings.TrimPrefix(path, "/assets/"))

	case path == "/sse":
		c.ServeSSE(w, r)

	case path == "/api/snapshot":
		c.handleAPISnapshot(w, r)

	case path == "/metrics":
		c.handleMetricsPage(w, r)

	case path == "/debug/file":
		c.handleDebugFile(w, r)

	case path == "/docs":
		c.handleDocsLanding(w, r)

	case strings.HasPrefix(path, "/docs/"):
		// /docs/{catalog}           → developer docs or operator CRD list
		// /docs/{catalog}/{crd}     → CRD docs page
		parts := strings.SplitN(strings.TrimPrefix(path, "/docs/"), "/", 2)
		if len(parts) == 2 {
			c.handleCRDDocs(w, r, parts[0], parts[1])
		} else if len(parts) == 1 && parts[0] != "" {
			c.handleCatalogDocs(w, r, parts[0])
		} else {
			c.handleDocsLanding(w, r)
		}

	case path == "/api/serve/apply" && r.Method == http.MethodPost:
		c.handleServeApply(w, r)

	case strings.HasPrefix(path, "/api/serve/schema/"):
		target := strings.TrimPrefix(path, "/api/serve/schema/")
		c.handleServeSchema(w, r, target)

	case strings.HasPrefix(path, "/catalog/"):
		// Strip leading slash and split
		parts := strings.Split(strings.Trim(path, "/"), "/")
		c.routeCatalog(w, r, parts)
	case path == "/api/instances" && r.Method == http.MethodGet:
		c.handleListInstances(w, r)
	case path == "/api/instances" && r.Method == http.MethodPost:
		c.handleAddInstance(w, r)
	case strings.HasPrefix(path, "/api/instances/") && r.Method == http.MethodPut:
		c.handleUpdateInstance(w, r)
	case strings.HasPrefix(path, "/api/instances/") && r.Method == http.MethodDelete:
		c.handleDeleteInstance(w, r)
	default:
		c.handleNotFound(w, r)
	}
}

// routeCatalog dispatches /catalog/** paths.
// parts has no leading slash: ["catalog", "{name}", ...]
func (c *Console) routeCatalog(w http.ResponseWriter, r *http.Request, parts []string) {
	// parts[0] == "catalog"
	if len(parts) < 2 {
		c.handleNotFound(w, r)
		return
	}

	catalogName := parts[1]

	switch {
	// /catalog/{name}/crd/{crd}/cr[/...]
	case len(parts) >= 5 && parts[2] == "crd" && parts[4] == "cr":
		crdName := parts[3]
		inst, ok := c.instanceByCatalogName(catalogName)
		if !ok {
			c.handleNotFound(w, r)
			return
		}
		c.routeCR(w, r, inst.URL, inst.Catalog.Name, crdName, parts[4:])

	// /catalog/{name}/crd/{crd}/raw  or  /catalog/{name}/crd/{crd}/enriched
	case len(parts) == 5 && parts[2] == "crd" && (parts[4] == "raw" || parts[4] == "enriched"):
		c.handleProxyCRDSpec(w, r, catalogName, parts[3], parts[4])

	// /catalog/{name}/crd/{crd}
	case len(parts) >= 4 && parts[2] == "crd":
		c.handleCRDDetail(w, r, catalogName, parts[3])

	// /catalog/{name}/raw  or  /catalog/{name}/enriched
	case len(parts) == 3 && (parts[2] == "raw" || parts[2] == "enriched"):
		c.handleProxyCatalogSpec(w, r, catalogName, parts[2])

	// /catalog/{name}
	default:
		c.handleCatalogPanel(w, r, catalogName)
	}
}

// routeCR dispatches CR sub-paths.
// crParts: ["cr"], ["cr", "{name}"], ["cr", "{ns}", "{name}"]
func (c *Console) routeCR(w http.ResponseWriter, r *http.Request, instanceURL, catalogName, crdName string, crParts []string) {
	// crParts[0] == "cr"
	switch len(crParts) {
	case 1:
		// /cr → list
		c.handleCRList(w, r, instanceURL, catalogName, crdName)
	case 2:
		if crParts[1] == "create" {
			c.handleServeCreateForm(w, r, instanceURL, catalogName, crdName)
			return
		}
		// /cr/{name} → cluster-scoped detail
		c.handleCRDetail(w, r, instanceURL, catalogName, crdName, "", crParts[1])
	case 3:
		// /cr/{ns}/{name} → namespaced detail
		c.handleCRDetail(w, r, instanceURL, catalogName, crdName, crParts[1], crParts[2])
	default:
		c.handleNotFound(w, r)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Raw / Enriched proxy handlers
// ─────────────────────────────────────────────────────────────────────────────

// handleProxyCRDSpec proxies GET /catalog/{crd}/raw or /catalog/{crd}/enriched
// from the Inrun runtime and returns the JSON body to the browser.
func (c *Console) handleProxyCRDSpec(w http.ResponseWriter, r *http.Request, catalogName, crdName, kind string) {
	inst, ok := c.instanceByCatalogName(catalogName)
	if !ok {
		http.Error(w, "catalog not found", http.StatusNotFound)
		return
	}
	target := inst.URL + "/catalog/" + crdName + "/" + kind
	c.proxyJSON(w, target)
}

// handleProxyCatalogSpec proxies GET /catalog/raw or /catalog/enriched
// from the Inrun runtime and returns the JSON body to the browser.
func (c *Console) handleProxyCatalogSpec(w http.ResponseWriter, r *http.Request, catalogName, kind string) {
	inst, ok := c.instanceByCatalogName(catalogName)
	if !ok {
		http.Error(w, "catalog not found", http.StatusNotFound)
		return
	}
	target := inst.URL + "/catalog/" + kind
	c.proxyJSON(w, target)
}

// proxyJSON fetches target and pipes the JSON response body to w.
func (c *Console) proxyJSON(w http.ResponseWriter, target string) {
	resp, err := http.Get(target) //nolint:gosec
	if err != nil {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body) //nolint:errcheck
}

// ─────────────────────────────────────────────────────────────────────────────
// Page handlers
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Live snapshot API — used by JS for partial DOM updates (no full-page reload)
// ─────────────────────────────────────────────────────────────────────────────

type StatsSnap struct {
	TotalCatalogs   int `json:"totalCatalogs"`
	HealthyCatalogs int `json:"healthyCatalogs"`
	TotalCRDs       int `json:"totalCRDs"`
	TotalWorkers    int `json:"totalWorkers"`
	TotalResources  int `json:"totalResources"`
}

type CatalogSnap struct {
	Name           string `json:"name"`
	Healthy        bool   `json:"healthy"`
	TotalCRDs      int    `json:"totalCRDs"`
	HealthyCRDs    int    `json:"healthyCRDs"`
	TotalWorkers   int    `json:"totalWorkers"`
	TotalResources int    `json:"totalResources"`
}

type SnapshotData struct {
	Stats    StatsSnap     `json:"stats"`
	Catalogs []CatalogSnap `json:"catalogs"`
	Updated  string        `json:"updated"`
}

func (c *Console) handleAPISnapshot(w http.ResponseWriter, _ *http.Request) {
	c.mu.RLock()
	insts := make([]*Instance, 0, len(c.instances))
	for _, inst := range c.instances {
		if inst.Catalog != nil {
			insts = append(insts, inst)
		}
	}
	c.mu.RUnlock()

	sort.Slice(insts, func(i, j int) bool {
		return insts[i].Catalog.Name < insts[j].Catalog.Name
	})

	stats := StatsSnap{}
	var catalogs []CatalogSnap

	for _, inst := range insts {
		cat := inst.Catalog
		healthyCRDs := 0
		for _, crd := range cat.CRDs {
			if crd.Healthy {
				healthyCRDs++
			}
		}
		stats.TotalCatalogs++
		stats.TotalCRDs += len(cat.CRDs)
		stats.TotalWorkers += sumWorkers(cat.CRDs)
		stats.TotalResources += sumResources(cat.CRDs)
		if cat.Healthy {
			stats.HealthyCatalogs++
		}
		catalogs = append(catalogs, CatalogSnap{
			Name:           cat.Name,
			Healthy:        cat.Healthy,
			TotalCRDs:      len(cat.CRDs),
			HealthyCRDs:    healthyCRDs,
			TotalWorkers:   sumWorkers(cat.CRDs),
			TotalResources: sumResources(cat.CRDs),
		})
	}

	if catalogs == nil {
		catalogs = []CatalogSnap{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(SnapshotData{
		Stats:    stats,
		Catalogs: catalogs,
		Updated:  time.Now().UTC().Format(time.RFC3339),
	})
}

func (c *Console) handleIndex(w http.ResponseWriter, _ *http.Request) {
	c.mu.RLock()
	insts := make([]*Instance, 0, len(c.instances))
	for _, inst := range c.instances {
		if inst.Catalog != nil {
			insts = append(insts, inst)
		}
	}
	c.mu.RUnlock()

	sort.Slice(insts, func(i, j int) bool {
		return insts[i].Catalog.Name < insts[j].Catalog.Name
	})

	var summaries []CatalogSummary
	totalCRDs, totalWorkers, totalResources, healthyCatalogs := 0, 0, 0, 0
	hasOperatorCatalogs := false
	clusterSeen := map[string]struct{}{}
	nsSeen := map[string]struct{}{}

	for _, inst := range insts {
		cat := inst.Catalog
		healthyCRDs := 0
		for _, crd := range cat.CRDs {
			if crd.Healthy {
				healthyCRDs++
			}
		}

		// Collect distinct namespaces from this catalog's namespace grouping.
		var nsKeys []string
		for ns := range cat.Namespaces {
			nsKeys = append(nsKeys, ns)
			nsSeen[ns] = struct{}{}
		}
		sort.Strings(nsKeys)

		if cat.ClusterName != "" {
			clusterSeen[cat.ClusterName] = struct{}{}
		}

		summary := CatalogSummary{
			Name:             cat.Name,
			Description:      cat.Description,
			Version:          cat.Version,
			Healthy:          cat.Healthy,
			TotalCRDs:        len(cat.CRDs),
			HealthyCRDs:      healthyCRDs,
			TotalWorkers:     sumWorkers(cat.CRDs),
			TotalResources:   sumResources(cat.CRDs),
			ClusterName:      cat.ClusterName,
			Namespaces:       nsKeys,
			NamespaceDetails: cat.Namespaces,
		}
		summaries = append(summaries, summary)
		hasOperatorCatalogs = true
		totalCRDs += len(cat.CRDs)
		totalWorkers += sumWorkers(cat.CRDs)
		totalResources += sumResources(cat.CRDs)
		if cat.Healthy {
			healthyCatalogs++
		}
	}

	allClusters := make([]string, 0, len(clusterSeen))
	for c := range clusterSeen {
		allClusters = append(allClusters, c)
	}
	sort.Strings(allClusters)

	allNamespaces := make([]string, 0, len(nsSeen))
	for ns := range nsSeen {
		allNamespaces = append(allNamespaces, ns)
	}
	sort.Strings(allNamespaces)

	c.renderTemplate(w, "index.html", IndexData{
		Catalogs:             summaries,
		TotalCatalogs:        len(summaries),
		HealthyCatalogs:      healthyCatalogs,
		TotalCRDs:            totalCRDs,
		TotalWorkers:         totalWorkers,
		TotalResources:       totalResources,
		AnyHealthy:           len(summaries) > 0,
		HasOperatorCatalogs:  hasOperatorCatalogs,
		InrunURLs:            strings.Join(c.urls, ", "),
		ConsoleVersion:       ccversion.Short(),
		AllClusters:          allClusters,
		AllNamespaces:        allNamespaces,
		EnableRuntimeManager: c.config.EnableRuntimeManager,
	})
}

func (c *Console) handleCatalogPanel(w http.ResponseWriter, r *http.Request, catalogName string) {
	inst, ok := c.instanceByCatalogName(catalogName)
	if !ok {
		// Try a fresh fetch once before giving up
		c.fetchAllCatalogs()
		inst, ok = c.instanceByCatalogName(catalogName)
		if !ok {
			c.handleNotFound(w, r)
			return
		}
	}

	cat := inst.Catalog

	ns := r.URL.Query().Get("ns")

	// Sort CRDs by name for consistent display
	sortedCRDs := make([]CRDSummary, len(cat.CRDs))
	copy(sortedCRDs, cat.CRDs)
	sort.Slice(sortedCRDs, func(i, j int) bool {
		return sortedCRDs[i].Name < sortedCRDs[j].Name
	})

	// When viewing a specific namespace, use that namespace's description and
	// version (set from each CRD's source Catalog metadata) rather than the
	// top-level Stack description which belongs to the whole bundle.
	description := cat.Description
	version := cat.Version
	if ns != "" {
		var filtered []CRDSummary
		for _, crd := range sortedCRDs {
			if crd.CatalogNamespace == ns {
				filtered = append(filtered, crd)
			}
		}
		sortedCRDs = filtered
		if nsSummary, ok := cat.Namespaces[ns]; ok {
			if nsSummary.Description != "" {
				description = nsSummary.Description
			}
			if nsSummary.Version != "" {
				version = nsSummary.Version
			}
		}
	}

	c.renderTemplate(w, "catalog.html", CatalogData{
		CRDs:               sortedCRDs,
		InrunReady:         cat.InrunReady,
		DeletionProtection: cat.DeletionProtection,
		TotalCRDs:          len(sortedCRDs),
		TotalWorkers:       sumWorkers(sortedCRDs),
		TotalResources:     sumResources(sortedCRDs),
		HealthyCount:       countHealthyCRDs(sortedCRDs),
		CatalogName:        cat.Name,
		CatalogDescription: description,
		CatalogHealthy:     cat.Healthy,
		CatalogVersion:     version,
		CatalogAuthor:      cat.Author,
		CatalogLicense:     cat.License,
		DegradedReason:     cat.DegradedReason,
		StatusCounts:       computeStatusCounts(sortedCRDs),
		RuntimeVersion:     cat.RuntimeVersion,
		ClusterName:        cat.ClusterName,
	})
}

func (c *Console) handleCRDDetail(w http.ResponseWriter, r *http.Request, catalogName, crdName string) {
	inst, ok := c.instanceByCatalogName(catalogName)
	if !ok {
		c.renderError(w, r, fmt.Sprintf("Catalog %q not found", catalogName))
		return
	}

	endpoints := endpointInfoFor(inst.Catalog, crdName)
	summary := summaryFor(inst.Catalog, crdName)
	crd, err := inst.Client.FetchCRDDetail(crdName, endpoints, summary)
	if err != nil {
		Warnf("fetch CRD %s: %v", crdName, err)
		// Render a degraded view rather than a hard error
		crd = &CRDDetail{
			Name:        crdName,
			State:       "offline",
			Healthy:     false,
			Description: "Unable to connect to Inrun runtime",
			GVK:         "unknown",
			LastError:   err.Error(),
		}
	}

	// Merge gateway-owned webhook stats when the runtime advertises a gateway.
	if inst.GatewayEndpoint != "" && crd.State != "offline" {
		if gwStats, err := FetchGatewayCRDStats(inst.GatewayEndpoint, crdName); err != nil {
			Warnf("gateway stats for %s: %v", crdName, err)
		} else if gwStats != nil {
			if gwStats.Admission != nil {
				crd.Admission = gwStats.Admission
			}
			if gwStats.Conversion != nil {
				crd.Conversion = gwStats.Conversion
			}
			if gwStats.DeletionProtection != nil {
				crd.DeletionProtection = gwStats.DeletionProtection
			}
			if gwStats.NamespaceProtection != nil {
				crd.NamespaceProtection = gwStats.NamespaceProtection
			}
		}
	}

	c.renderTemplate(w, "crd.html", map[string]interface{}{
		"CRD":         crd,
		"CatalogName": catalogName,
	})
}

// handleCRList renders the CR instance list for one CRD.
// Route: /catalog/{catalog}/crd/{crd}/cr
func (c *Console) handleCRList(w http.ResponseWriter, r *http.Request, instanceURL, catalogName, crdName string) {
	client := c.clientFor(instanceURL)

	list, err := client.FetchCRList(instanceURL, crdName)
	if err != nil {
		c.renderError(w, r, fmt.Sprintf("Could not load CR list for %s: \n%v", crdName, err))
		return
	}

	var serveEnabled bool
	var gatewayEndpoint string
	c.mu.RLock()
	if inst, ok := c.instances[instanceURL]; ok && inst.Catalog != nil {
		gatewayEndpoint = inst.GatewayEndpoint
		for _, crd := range inst.Catalog.CRDs {
			if strings.EqualFold(crd.Name, crdName) {
				serveEnabled = crd.ServeEnabled
				break
			}
		}
	}
	c.mu.RUnlock()

	c.renderTemplate(w, "cr_list.html", CRListView{
		CatalogName:     catalogName,
		Instance:        instanceURL,
		CRDName:         crdName,
		GVK:             list.GVK,
		Total:           list.Total,
		Items:           list.Items,
		BackURL:         fmt.Sprintf("/console/catalog/%s/crd/%s", catalogName, crdName),
		ServeEnabled:    serveEnabled,
		GatewayEndpoint: gatewayEndpoint,
	})
}

// handleServeSchema proxies GET /api/v1/schema?target=<target> from the gateway.
// The gateway token is stored server-side — the browser never sees it.
func (c *Console) handleServeSchema(w http.ResponseWriter, r *http.Request, target string) {
	inst := c.firstInstance()
	if inst == nil || inst.GatewayEndpoint == "" {
		http.Error(w, `{"error":"no gateway configured"}`, http.StatusServiceUnavailable)
		return
	}
	c.proxyServeRequest(w, r, inst.GatewayEndpoint+"/api/v1/schema?target="+url.QueryEscape(target), http.MethodGet, nil)
}

// handleServeApply proxies POST /api/v1/apply to the gateway.
func (c *Console) handleServeApply(w http.ResponseWriter, r *http.Request) {
	inst := c.firstInstance()
	if inst == nil || inst.GatewayEndpoint == "" {
		http.Error(w, `{"error":"no gateway configured"}`, http.StatusServiceUnavailable)
		return
	}
	c.proxyServeRequest(w, r, inst.GatewayEndpoint+"/api/v1/apply", http.MethodPost, r.Body)
}

// proxyServeRequest forwards a serve request to the gateway with the stored bearer token.
func (c *Console) proxyServeRequest(w http.ResponseWriter, _ *http.Request, target, method string, body io.Reader) {
	req, err := http.NewRequest(method, target, body) //nolint:noctx
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if c.gatewayToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.gatewayToken)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, `{"error":"gateway unreachable"}`, http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	// If the gateway returned plain text (e.g. "Unauthorized"), wrap it so the
	// browser always receives valid JSON regardless of gateway error format.
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		fmt.Fprintf(w, `{"error":%q}`, strings.TrimSpace(string(respBody)))
		return
	}
	w.Write(respBody) //nolint:errcheck
}

// firstInstance returns the first instance that has a loaded Catalog, or nil.
func (c *Console) firstInstance() *Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, inst := range c.instances {
		if inst.Catalog != nil {
			return inst
		}
	}
	return nil
}

// instanceByURL returns the Instance for the given URL, or nil.
func (c *Console) instanceByURL(instanceURL string) *Instance {
	c.mu.RLock()
	defer c.mu.RUnlock()
	inst := c.instances[instanceURL]
	return inst
}

// handleServeCreateForm renders the serve create form (GET) or applies the CR (POST).
// Route: /catalog/{catalog}/crd/{crd}/cr/create
func (c *Console) handleServeCreateForm(w http.ResponseWriter, r *http.Request, instanceURL, catalogName, crdName string) {
	backURL := fmt.Sprintf("/console/catalog/%s/crd/%s/cr", catalogName, crdName)

	inst := c.instanceByURL(instanceURL)
	if inst == nil || inst.Catalog == nil {
		http.Redirect(w, r, backURL, http.StatusSeeOther)
		return
	}

	var crdSummary *CRDSummary
	for i := range inst.Catalog.CRDs {
		if strings.EqualFold(inst.Catalog.CRDs[i].Name, crdName) {
			crdSummary = &inst.Catalog.CRDs[i]
			break
		}
	}
	if crdSummary == nil || !crdSummary.ServeEnabled || crdSummary.Target == "" {
		http.Redirect(w, r, backURL, http.StatusSeeOther)
		return
	}

	target := crdSummary.Target
	// Kind/APIVersion are display-only (page header) — the gateway builds
	// the CR from target, it doesn't need them from the form.
	kind, apiVersion := parseServeGVK(crdSummary.GVK)

	if r.Method == http.MethodPost {
		c.handleServeApplyForm(w, r, inst, target)
		return
	}

	sections, aliases, fetchErr := c.fetchAllServeFields(inst, target)
	data := ServeFormData{
		CatalogName:      catalogName,
		CRDName:          crdName,
		Target:           target,
		Aliases:          aliases,
		Kind:             kind,
		APIVersion:       apiVersion,
		BackURL:          backURL,
		Namespaced:       crdSummary.Namespaced,
		RequireServeName: crdSummary.RequireServeName,
		Sections:         sections,
	}
	if fetchErr != nil {
		data.Error = "Could not load schema: " + fetchErr.Error()
	}
	c.renderTemplate(w, "serve_form.html", data)
}

// FieldType defines the valid types for serve fields.
// These are the only types that are allowed for serve.fields fields.
type FieldType string

const (
	FieldTypeString  FieldType = "string"
	FieldTypeInteger FieldType = "integer"
	FieldTypeNumber  FieldType = "number"
	FieldTypeBoolean FieldType = "boolean"
	FieldTypeEnum    FieldType = "enum"
)

// serveFieldHint mirrors one entry of pkg/gateway/api.SchemaResponse.Fields
// (itself types.ServeFieldConfig) — the gateway's flat, caller-facing field
// contract. It doesn't distinguish where a field routes to (spec, label,
// annotation) — the gateway resolves that from the Catalog, not the caller.
type serveFieldHint struct {
	Label       string            `json:"label"`
	Placeholder string            `json:"placeholder"`
	Hint        string            `json:"hint"`
	Order       int               `json:"order"`
	Category    string            `json:"category"`
	Required    bool              `json:"required"`
	Disabled    string            `json:"disabled"`
	When        []json.RawMessage `json:"when"`
	Or          []json.RawMessage `json:"or"`
	Type        string            `json:"type"`
	Enum        []string          `json:"enum"`
}

// buildServeField builds an ServeField for one gateway schema field entry.
// Type/enum come from the field's own declared type — the gateway's flat
// schema doesn't distinguish where a field routes to (spec, label,
// annotation), so neither does the form.
func buildServeField(name string, hint serveFieldHint) ServeField {
	label := hint.Label
	if label == "" {
		label = name
		if len(label) > 0 {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
	}
	f := ServeField{
		Name:        name,
		Label:       label,
		Hint:        hint.Hint,
		Placeholder: hint.Placeholder,
		Required:    hint.Required,
		Category:    hint.Category,
		Disabled:    hint.Disabled,
	}
	// Encode when/or as JSON strings for the template to embed as data attributes.
	if len(hint.When) > 0 {
		if b, err := json.Marshal(hint.When); err == nil {
			f.WhenJSON = string(b)
		}
	}
	if len(hint.Or) > 0 {
		if b, err := json.Marshal(hint.Or); err == nil {
			f.OrJSON = string(b)
		}
	}
	switch {
	case hint.Type == string(FieldTypeEnum) && len(hint.Enum) > 0:
		f.InputType = "select"
		f.Enum = hint.Enum
	case hint.Type == string(FieldTypeBoolean):
		f.InputType = "checkbox"
	case hint.Type == string(FieldTypeInteger) || hint.Type == string(FieldTypeNumber):
		f.InputType = "number"
	default:
		f.InputType = "text"
	}
	return f
}

// fetchAllServeFields fetches the flat field schema for target from the gateway.
// It also returns the alias names declared for this CRD so the form can render
// a surface dropdown when more than one entry point exists.
func (c *Console) fetchAllServeFields(inst *Instance, target string) ([]ServeSection, []string, error) {
	if inst.GatewayEndpoint == "" {
		return nil, nil, fmt.Errorf("no gateway endpoint configured")
	}
	if c.gatewayToken == "" {
		return nil, nil, fmt.Errorf("GATEWAY_TOKEN not set")
	}

	req, _ := http.NewRequest(http.MethodGet, inst.GatewayEndpoint+"/api/v1/schema?target="+url.QueryEscape(target), nil) //nolint:noctx
	req.Header.Set("Authorization", "Bearer "+c.gatewayToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("%s", strings.TrimSpace(string(body)))
	}

	var schemaResp struct {
		Aliases []string                  `json:"aliases"`
		Fields  map[string]serveFieldHint `json:"fields"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&schemaResp); err != nil {
		return nil, nil, err
	}

	type orderedField struct {
		field ServeField
		order int
	}
	var ordered []orderedField
	for name, hint := range schemaResp.Fields {
		ordered = append(ordered, orderedField{field: buildServeField(name, hint), order: hint.Order})
	}

	sort.Slice(ordered, func(i, j int) bool {
		oi, oj := ordered[i].order, ordered[j].order
		if oi == 0 && oj != 0 {
			return false
		}
		if oi != 0 && oj == 0 {
			return true
		}
		if oi != oj {
			return oi < oj
		}
		return ordered[i].field.Name < ordered[j].field.Name
	})
	var sections []ServeSection
	for _, o := range ordered {
		f := o.field
		title := f.Category
		if title == "" {
			title = "Fields"
		}
		if len(sections) == 0 || sections[len(sections)-1].Title != title {
			sections = append(sections, ServeSection{Title: title})
		}
		sections[len(sections)-1].Fields = append(sections[len(sections)-1].Fields, f)
	}
	return sections, schemaResp.Aliases, nil
}

// handleServeApplyForm processes the serve form POST: forwards the flat
// target-mode payload the form submitted to the Gateway API. The
// gateway builds the CR from target — this handler doesn't construct one.
func (c *Console) handleServeApplyForm(w http.ResponseWriter, r *http.Request, inst *Instance, target string) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, `{"error":"invalid request body"}`)
		return
	}
	// If the form submitted a surface (alias or primary), use it. The gateway
	// validates the surface and rejects unknown targets, so we don't need to
	// whitelist here. Fall back to the route-resolved primary target when the
	// form omits it (shouldn't happen, but safe default).
	if submitted, ok := body["target"].(string); !ok || submitted == "" {
		body["target"] = target
	}

	payload, _ := json.Marshal(body)
	applyURL := inst.GatewayEndpoint + "/api/v1/apply"
	if r.URL.Query().Get("dryRun") == "true" {
		applyURL += "?dryRun=true"
	}
	c.proxyServeRequest(w, r, applyURL, http.MethodPost, bytes.NewReader(payload))
}

// parseServeGVK splits "group/version, Kind=Kind" into (kind, apiVersion).
func parseServeGVK(gvk string) (kind, apiVersion string) {
	parts := strings.SplitN(gvk, ",", 2)
	if len(parts) == 2 {
		apiVersion = strings.TrimSpace(parts[0])
		kind = strings.TrimPrefix(strings.TrimSpace(parts[1]), "Kind=")
	}
	return
}

// handleCRDetail renders one CR instance with its children and events.
// Route: /catalog/{catalog}/crd/{crd}/cr/{ns}/{name}  (namespace may be empty)
func (c *Console) handleCRDetail(w http.ResponseWriter, r *http.Request, instanceURL, catalogName, crdName, namespace, name string) {
	client := c.clientFor(instanceURL)

	detail, err := client.FetchCRDetail(instanceURL, crdName, namespace, name)
	if err != nil {
		if namespace != "" {
			c.renderError(w, r, fmt.Sprintf("Could not load CR '%s/%s': \n%v", namespace, name, err))
		} else {
			c.renderError(w, r, fmt.Sprintf("Could not load CR '%s': \n%v", name, err))
		}
		return
	}

	// Events are best-effort — never block or error on failure
	var events []CREvent
	var eventTotal int
	if evResp, err := client.FetchCREvents(instanceURL, crdName, namespace, name); err == nil {
		events = evResp.Events
		eventTotal = evResp.Total
	}

	// Extract phase from status for template convenience
	phase := ""
	if detail.Status != nil {
		if p, ok := detail.Status["phase"].(string); ok {
			phase = p
		}
	}

	c.renderTemplate(w, "cr_detail.html", CRDetailView{
		CatalogName: catalogName,
		Instance:    instanceURL,
		CRDName:     crdName,
		CR:          *detail,
		Events:      events,
		EventTotal:  eventTotal,
		Phase:       phase,
		ChildGroups: normalizeChildGroups(detail.Children),
		BackURL:     fmt.Sprintf("/console/catalog/%s/crd/%s", catalogName, crdName),
	})
}

func (c *Console) handleListInstances(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	type InstanceDTO struct {
		URL       string    `json:"url"`
		Healthy   bool      `json:"healthy"`
		LastError string    `json:"lastError,omitempty"`
		LastCheck time.Time `json:"lastCheck"`
	}

	out := make([]InstanceDTO, 0, len(c.instances))
	for _, inst := range c.instances {
		out = append(out, InstanceDTO{
			URL:       inst.URL,
			Healthy:   inst.Healthy,
			LastError: inst.LastError,
			LastCheck: inst.LastCheck,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"instances": out,
	})
}

func (c *Console) handleAddInstance(w http.ResponseWriter, r *http.Request) {
	var req struct{ URL string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	if err := c.AddInstance(req.URL); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

func (c *Console) handleUpdateInstance(w http.ResponseWriter, r *http.Request) {
	// Path: /api/instances/encodedOldURL
	encoded := strings.TrimPrefix(r.URL.Path, "/api/instances/")
	oldURL, err := url.PathUnescape(encoded)
	if err != nil {
		http.Error(w, `{"error":"invalid URL"}`, http.StatusBadRequest)
		return
	}
	var req struct{ URL string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}
	newURL := normalizeURL(req.URL)
	if err := c.UpdateInstance(oldURL, newURL); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (c *Console) handleDeleteInstance(w http.ResponseWriter, r *http.Request) {
	encoded := strings.TrimPrefix(r.URL.Path, "/api/instances/")
	urlStr, err := url.PathUnescape(encoded)
	if err != nil {
		http.Error(w, `{"error":"invalid URL"}`, http.StatusBadRequest)
		return
	}
	if err := c.DeleteInstance(urlStr); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// normalizeURL adds scheme and removes trailing slash.
func normalizeURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "http://" + u
	}
	return strings.TrimSuffix(u, "/")
}

// ─────────────────────────────────────────────────────────────────────────────
// Asset and utility handlers
// ─────────────────────────────────────────────────────────────────────────────

func (c *Console) serveAsset(w http.ResponseWriter, r *http.Request, filePath string) {
	data, err := Assets.ReadFile("assets/" + filePath)
	if err != nil {
		c.handleNotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(filePath, ".css"):
		w.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(filePath, ".js"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(filePath, ".png"):
		w.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(filePath, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Write(data)
}

func (c *Console) handleMetricsPage(w http.ResponseWriter, _ *http.Request) {
	c.renderTemplate(w, "metrics.html", nil)
}

func (c *Console) handleDebugFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		filePath = "assets/static/inrun-mark.svg"
	}
	fmt.Fprintf(w, "Reading: %s\n\n", filePath)
	data, err := Assets.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(w, "ERROR: %v\n", err)
		return
	}
	fmt.Fprintf(w, "OK: %d bytes\n", len(data))

	entries, _ := Assets.ReadDir("assets/static")
	fmt.Fprintf(w, "\nassets/static/:\n")
	for _, e := range entries {
		fmt.Fprintf(w, "  %s\n", e.Name())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Aggregate helpers
// ─────────────────────────────────────────────────────────────────────────────

func sumWorkers(crds []CRDSummary) int {
	n := 0
	for _, c := range crds {
		n += c.Workers
	}
	return n
}

func sumResources(crds []CRDSummary) int {
	n := 0
	for _, c := range crds {
		n += c.ResourceCount
	}
	return n
}

func countHealthyCRDs(crds []CRDSummary) int {
	n := 0
	for _, c := range crds {
		if c.Healthy {
			n++
		}
	}
	return n
}

func computeStatusCounts(crds []CRDSummary) StatusCounts {
	var sc StatusCounts
	for _, c := range crds {
		switch c.State {
		case "healthy":
			sc.Healthy++
		case "degraded":
			sc.Degraded++
		case "started":
			sc.Started++
		case "gated":
			sc.Gated++
		default:
			sc.Pending++
		}
	}
	return sc
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
