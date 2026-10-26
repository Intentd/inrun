package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"

	"k8s.io/client-go/kubernetes"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/gateway/webhook/admission"
	"github.com/intentd/inrun/pkg/gateway/webhook/conversion"
	"github.com/intentd/inrun/pkg/gateway/webhook/protection"
	"github.com/intentd/inrun/pkg/gateway/webhook/registration"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/util/kube"
)

var _ domain.Component = (*Server)(nil)

// certManagerIface is the part of certmanager.Manager that removes the TLS
// Secret on shutdown.
type certManagerIface interface {
	DeleteCertificateAndSecret(ctx context.Context, namespace, secretName string) error
}

// Server serves the admission, conversion and protection webhooks over
// HTTPS. It registers their configurations at startup and removes them on
// shutdown when the Blueprint asks; the housekeeper keeps them in sync in
// between.
//
// Created with New and started as a domain.Component by
// pkg/gateway/app, after the health server so /ready is live before
// registration runs.
type Server struct {
	server  *http.Server
	mux     *http.ServeMux
	started atomic.Bool

	tlsCert   string
	tlsKey    string
	httpsPort string

	client  kubernetes.Interface
	bp      *blueprint.Blueprint
	cfg     *config.Config
	hookReg registration.Options

	admission  *admission.Handler
	conversion *conversion.Handler
	protection *protection.Handler

	// certMgr removes the generated TLS Secret on shutdown. Nil when the
	// certificate is configured rather than generated.
	certMgr certManagerIface
}

// New builds the server and its handlers from the Blueprint and
// config. It does no I/O; Start serves and registers.
func New(client kubernetes.Interface, bp *blueprint.Blueprint, cfg *config.Config) *Server {
	window := bp.ConversionWindow()
	return &Server{
		mux:        http.NewServeMux(),
		tlsCert:    cfg.Security().Webhooks.TLSCert,
		tlsKey:     cfg.Security().Webhooks.TLSKey,
		httpsPort:  cfg.HTTPSPort(),
		client:     client,
		bp:         bp,
		cfg:        cfg,
		hookReg:    registration.NewOptions(bp, cfg),
		admission:  admission.New(client, bp, cfg, window),
		conversion: conversion.New(bp, window),
		protection: protection.New(bp),
	}
}

// SetCertManager gives the server the cert manager that removes the generated
// TLS Secret on shutdown. Not called when the certificate is configured.
func (ws *Server) SetCertManager(m certManagerIface) {
	ws.certMgr = m
}

// Start serves the webhooks the Blueprint declares over HTTPS and registers
// their configurations. Outside a pod, or with no webhooks declared, it does
// nothing.
func (ws *Server) Start(context.Context) error {
	ws.started.Store(true)

	if !kube.IsRunningInPod() {
		logger.Debug().Msg("not running in cluster — skipping webhook HTTPS setup")
		return nil
	}

	if !ws.route() {
		logger.Debug().Msg("no webhook endpoints declared — HTTPS server not started")
		return nil
	}

	if ws.tlsCert == "" {
		return fmt.Errorf("webhook server: TLS_CERT is required when webhook endpoints are active")
	}
	if ws.tlsKey == "" {
		return fmt.Errorf("webhook server: TLS_KEY is required when webhook endpoints are active")
	}

	addr := ws.httpsPort
	if !strings.HasPrefix(addr, ":") {
		addr = ":" + addr
	}
	ws.server = &http.Server{Addr: addr, Handler: ws.mux}

	go func() {
		logger.Info().Str("addr", addr).Str("cert", ws.tlsCert).Msg("webhook HTTPS server listening")
		if err := ws.server.ListenAndServeTLS(ws.tlsCert, ws.tlsKey); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Str("addr", addr).Msg("webhook HTTPS server error")
		}
	}()

	ws.register()
	return nil
}

// route adds an endpoint for each webhook the Blueprint declares, and reports
// whether there is any.
func (ws *Server) route() bool {
	bp := ws.bp
	endpoints := []struct {
		on      bool
		path    string
		handler http.HandlerFunc
	}{
		{bp.DeletionProtectionActive(), "/deletion-protection", ws.protection.Deletion},
		{bp.NamespaceProtectionActive(), "/namespace-protection", ws.protection.Namespace},
		{bp.IsStrictModeEnabled(), "/strict-mode-protection", ws.protection.StrictMode},
		{bp.HasConversionPaths(), "/convert", ws.conversion.Convert},
		{bp.HasValidationRules(), "/validate", ws.admission.Validate},
		{bp.HasMutationRules(), "/mutate", ws.admission.Mutate},
	}

	routed := false
	for _, e := range endpoints {
		if !e.on {
			continue
		}
		ws.mux.HandleFunc(e.path, e.handler)
		logger.Info().Str("endpoint", e.path).Msg("webhook endpoint registered")
		routed = true
	}
	return routed
}

// Shutdown stops the HTTPS server and removes the configurations the
// Blueprint asks to clean up on shutdown.
func (ws *Server) Shutdown(ctx context.Context) {
	if ws.server == nil {
		return
	}
	if err := ws.server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("webhook HTTPS server shutdown error")
	}
	ws.cleanup(ctx)
}

// Name returns the component name.
func (ws *Server) Name() string { return "webhook server" }

// Started reports whether Start has been called.
func (ws *Server) Started() bool { return ws.started.Load() }
