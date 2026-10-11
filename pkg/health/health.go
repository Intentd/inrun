package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var _ domain.Component = (*HealthServer)(nil)

// HealthServer serves Inrun's HTTP health, readiness, and metrics endpoints.
// It is intentionally minimal — all webhook and admission logic lives in pkg/webhook.
//
// Routes are registered before Start() via Register(). Start() binds the HTTP port
// and begins serving. Shutdown() drains and stops the server.
type HealthServer struct {
	name string

	// HTTP server for health, readiness, metrics, and Catalog API routes.
	server *http.Server
	mux    *http.ServeMux

	// Kubernetes probe state flags.
	started atomic.Bool // internal startup indicator
	healthy atomic.Bool
	ready   atomic.Bool
	startup atomic.Bool // Kubernetes startupProbe indicator

	httpPort  string
	client    string // operator name, used in probe response bodies
	logLevel  string
	startTime time.Time
}

// NewHealthServer constructs a HealthServer. No I/O is performed.
// Routes must be registered via Register() before calling Start().
func NewHealthServer(kfg *config.Config) *HealthServer {
	hs := &HealthServer{
		name:     "health server",
		client:   kfg.Inrun().Name(),
		httpPort: kfg.Health().Port(),
		logLevel: kfg.Inrun().LogLevel(),
		mux:      http.NewServeMux(),
	}
	hs.ready.Store(false)
	hs.started.Store(false)
	hs.healthy.Store(false)
	return hs
}

// Register adds a route to the health server mux.
// Must be called before Start() — routes registered after Start() are not
// guaranteed to be visible.
func (hs *HealthServer) Register(path string, handler http.HandlerFunc) {
	hs.mux.Handle(path, hs.logRoutesMiddleware(handler))
}

// Start binds the HTTP port, registers the standard probe endpoints, and begins
// serving. After this call, /startup, /health, /ready, and /metrics are live.
func (h *HealthServer) Start(ctx context.Context) error {
	h.startTime = time.Now()

	if !strings.HasPrefix(h.httpPort, ":") {
		h.httpPort = ":" + h.httpPort
	}

	if strings.ToLower(h.logLevel) == "debug" {
		h.mux.Handle("/startup", h.logRoutesMiddleware(http.HandlerFunc(h.startupHandler)))
		h.mux.Handle("/health", h.logRoutesMiddleware(http.HandlerFunc(h.healthHandler)))
		h.mux.Handle("/ready", h.logRoutesMiddleware(http.HandlerFunc(h.readyHandler)))
	} else {
		h.mux.HandleFunc("/startup", h.startupHandler)
		h.mux.HandleFunc("/health", h.healthHandler)
		h.mux.HandleFunc("/ready", h.readyHandler)
	}
	h.mux.Handle("/metrics", promhttp.Handler())
	h.registerQueryHandler()

	ln, err := net.Listen("tcp", h.httpPort)
	if err != nil {
		return fmt.Errorf("health server: cannot bind %s: %w", h.httpPort, err)
	}

	h.server = &http.Server{
		Addr:    h.httpPort,
		Handler: h.mux,
	}

	h.started.Store(true)
	h.healthy.Store(true)

	go func() {
		logger.Info().Str("port", h.httpPort).Msg("health server listening")
		if err := h.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Str("port", h.httpPort).Msg("health server error")
		}
	}()

	h.SetStartupComplete()
	h.ready.Store(true)
	return nil
}

// Shutdown gracefully stops the HTTP server and marks the server as not ready.
func (h *HealthServer) Shutdown(ctx context.Context) {
	h.ready.Store(false)
	h.healthy.Store(false)

	if h.server != nil {
		if err := h.server.Shutdown(ctx); err != nil {
			logger.Error().Err(err).Msg("http server shutdown error")
		}
	}
}

// Name returns the component name.
func (h *HealthServer) Name() string { return h.name }

// StartupComplete reports whether the startup sequence has finished.
func (h *HealthServer) StartupComplete() bool { return h.startup.Load() }

// SetStartupComplete marks the startup sequence as finished.
func (h *HealthServer) SetStartupComplete() { h.startup.Store(true) }

// SetReady marks the server as ready to serve traffic.
func (h *HealthServer) SetReady() { h.ready.Store(true) }

// Degraded transitions the server out of ready state without marking it unhealthy.
func (h *HealthServer) Degraded() {
	if h.ready.Load() {
		h.ready.Store(false)
	}
}

// Unhealthy marks the server as unhealthy, signaling a fatal condition.
func (h *HealthServer) Unhealthy() { h.healthy.Store(false) }

// Started reports whether the HTTP server has begun serving.
func (h *HealthServer) Started() bool { return h.started.Load() }

// SetStarted marks the server as having begun serving.
func (h *HealthServer) SetStarted() { h.started.Store(true) }

// Healthy reports whether the server is healthy.
func (h *HealthServer) Healthy() bool { return h.healthy.Load() }

// Ready reports whether the server is ready for health checks.
func (h *HealthServer) Ready() bool { return h.ready.Load() }

// Uptime returns human-readable uptime since the server started.
func (h *HealthServer) Uptime() string {
	if h.startTime.IsZero() {
		return "unknown"
	}
	return time.Since(h.startTime).Round(time.Second).String()
}
