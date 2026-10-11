package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
)

// summaryFor returns the CRDSummary for a named CRD from a cached CatalogResponse.
// Returns nil when the catalog is nil or the CRD is not found.
func summaryFor(cat *CatalogResponse, crdName string) *CRDSummary {
	if cat == nil {
		return nil
	}
	for i := range cat.CRDs {
		if strings.EqualFold(cat.CRDs[i].Name, crdName) {
			return &cat.CRDs[i]
		}
	}
	return nil
}

// synthHealthFromSummary builds a CRDHealth from the /catalog summary so that
// CRDs with a disabled health endpoint still show correct state and uptime in
// the Console instead of a zero placeholder.
func synthHealthFromSummary(name string, s *CRDSummary) *CRDHealth {
	if s == nil {
		return &CRDHealth{Name: name, State: "started", Started: true}
	}
	return &CRDHealth{
		Name:        name,
		State:       s.State,
		Healthy:     s.Healthy,
		Started:     s.Started,
		Pending:     s.Pending,
		Uptime:      s.Uptime,
		ErrorRate:   s.ErrorRate,
		QueueDepth:  s.QueueDepth,
		Gated:       s.Gated,
		GatedReason: s.GatedReason,
	}
}

// synthInfoFromSummary builds a minimal CRDInfo from the /catalog summary for
// CRDs whose info endpoint is disabled.
func synthInfoFromSummary(name string, s *CRDSummary) *CRDInfo {
	if s == nil {
		return &CRDInfo{Name: name}
	}
	return &CRDInfo{
		Name:          name,
		Description:   s.Description,
		Mode:          s.Mode,
		GVK:           s.GVK,
		GVR:           s.GVR,
		Namespaced:    s.Namespaced,
		Namespace:     s.Namespace,
		Workers:       s.Workers,
		WorkersActive: int32(s.WorkersActive),
		QueueDepth:    s.QueueDepth,
		MaxDepth:      s.MaxDepth,
		ResourceCount: s.ResourceCount,
		ErrorRate:     s.ErrorRate,
	}
}

// endpointInfoFor returns the EndpointInfo for a named CRD from a cached
// CatalogResponse. When the catalog is nil or the CRD is not found it returns
// a fully-enabled default so the caller retains normal fetch behaviour.
func endpointInfoFor(cat *CatalogResponse, crdName string) EndpointInfo {
	if cat == nil {
		return EndpointInfo{HealthEnabled: true, InfoEnabled: true}
	}
	for _, c := range cat.CRDs {
		if strings.EqualFold(c.Name, crdName) {
			return c.Endpoints
		}
	}
	return EndpointInfo{HealthEnabled: true, InfoEnabled: true}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rendering helpers
// ─────────────────────────────────────────────────────────────────────────────

// renderTemplate parses and executes a named template from the embedded FS.
// Writes directly to w. On error, renders a 500 page.
func (c *Console) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	tmpl, err := template.New(name).Funcs(templateFuncs).ParseFS(Assets, TemplateDir+"/"+name)
	if err != nil {
		Errorf("parse %s: %v", name, err)
		c.renderError(w, nil, fmt.Sprintf("Template error: %v", err))
		return
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, data); err != nil {
		Errorf("execute %s: %v", name, err)
		c.renderError(w, nil, fmt.Sprintf("Render error: %v", err))
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

// renderError renders an inline error page. r is optional (may be nil).
func (c *Console) renderError(w http.ResponseWriter, _ *http.Request, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)

	message = strings.ReplaceAll(message, "\n", "<br>")

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Error – Inrun Console</title>
<link rel="stylesheet" href="/console/assets/static/css/style.css">
<link rel="icon" type="image/svg+xml" href="/console/assets/static/inrun-mark.svg">
<link rel="stylesheet" href="/console/assets/static/css/style.css">
<script>(function(){var t=localStorage.getItem('console-theme')||'dark';document.documentElement.setAttribute('data-theme',t);})();</script>
</head>
<body style="display:flex;align-items:center;justify-content:center;min-height:100vh;background:var(--bg-base)">
  <div style="text-align:center;padding:40px;max-width:480px">
    <div style="font-size:36px;margin-bottom:16px;opacity:0.6">⚠</div>
    <h1 style="font-size:20px;font-weight:700;color:var(--text-primary);margin-bottom:8px">Something went wrong</h1>
    <p style="font-size:12px;font-family:monospace;color:var(--text-muted);margin-bottom:24px;word-break:break-all">%s</p>
    <a href="/console" style="display:inline-flex;align-items:center;gap:6px;padding:8px 16px;background:var(--accent);color:#fff;border-radius:6px;text-decoration:none;font-size:13px">
      ← Console
    </a>
  </div>
</body></html>`, message)
}

// handleNotFound renders a 404 page.
func (c *Console) handleNotFound(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>404 – Inrun Console</title>
<link rel="icon" type="image/svg+xml" href="/console/assets/static/inrun-mark.svg">
<link rel="stylesheet" href="/console/assets/static/css/style.css">
<script>(function(){var t=localStorage.getItem('console-theme')||'dark';document.documentElement.setAttribute('data-theme',t);})();</script>
</head>
<body style="display:flex;align-items:center;justify-content:center;min-height:100vh;background:var(--bg-base)">
  <div style="text-align:center;padding:40px;max-width:400px">
    <div style="font-size:60px;font-weight:700;color:var(--text-muted);margin-bottom:8px">404</div>
    <h1 style="font-size:18px;font-weight:600;color:var(--text-primary);margin-bottom:8px">Page not found</h1>
    <p style="font-size:13px;color:var(--text-muted);margin-bottom:24px">The page you're looking for doesn't exist.</p>
    <a href="/console" style="display:inline-flex;align-items:center;gap:6px;padding:8px 16px;background:var(--accent);color:#fff;border-radius:6px;text-decoration:none;font-size:13px">
      ← Back to Console
    </a>
  </div>
</body></html>`)
}

// encodeInstance encodes an instance URL for use in URL paths.
// Strips http:// and replaces : with - for safe path embedding.
func encodeInstance(url string) string {
	s := strings.TrimPrefix(url, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.ReplaceAll(s, ":", "-")
	s = strings.TrimSuffix(s, "/")
	return s
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data == nil {
		return
	}

	// Encode response as JSON
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

}

type H map[string]interface{}
