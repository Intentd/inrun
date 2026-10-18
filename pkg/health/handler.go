// pkg/health/handlers.go
package health

import (
	"net/http"
	"time"

	"github.com/intentd/inrun/pkg/util/httpapi"

	"github.com/intentd/inrun/pkg/logger"
)

// startupHandler — GET /startup
// Returns 200 once the controller has fully started, 503 while still booting.
// Standard Kubernetes startup probe endpoint.
// Prevents liveness/readiness from running until the process is initialized.
func (h *HealthServer) startupHandler(w http.ResponseWriter, r *http.Request) {
	if !h.StartupComplete() {
		httpapi.WriteJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":  "starting",
			"service": h.client,
		})
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "started",
		"service": h.client,
		"uptime":  h.Uptime(),
		"started": h.startTime.Format(time.RFC3339),
	})
}

// healthHandler — GET /health
// Returns 200 when the controller is healthy, 500 when not.
// Standard Kubernetes liveness probe endpoint.
func (h *HealthServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	if !h.healthy.Load() {
		httpapi.WriteJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":  "unhealthy",
			"service": h.client,
		})
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": h.client,
		"uptime":  h.Uptime(),
		"started": h.startTime.Format(time.RFC3339),
	})
}

// readyHandler — GET /ready
// Returns 200 when the controller is ready to serve traffic, 503 when not.
// Standard Kubernetes readiness probe endpoint.
// Not ready during startup (before informer caches sync) and during shutdown.
func (h *HealthServer) readyHandler(w http.ResponseWriter, r *http.Request) {
	if !h.ready.Load() {
		httpapi.WriteJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":  "not ready",
			"service": h.client,
		})
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ready",
		"service": h.client,
		"uptime":  h.Uptime(),
		"started": h.startTime.Format(time.RFC3339),
	})
}

// logRoutesMiddleware logs every request through the health server.
func (h *HealthServer) logRoutesMiddleware(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("userAgent", r.UserAgent()).
			Dur("duration", time.Since(start)).
			Msg("health request")
	}
}
