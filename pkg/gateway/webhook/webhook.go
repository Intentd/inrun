package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/intentd/inrun/pkg/util/kube"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/labels"
	"github.com/intentd/inrun/pkg/logger"
	"k8s.io/client-go/kubernetes"
)

var _ domain.Component = (*WebhookServer)(nil)

// certManagerIface is the subset of certmanager.Manager that WebhookServer needs
// for TLS secret cleanup on graceful shutdown.
type certManagerIface interface {
	DeleteCertificateAndSecret(ctx context.Context, namespace, secretName string) error
}

// WebhookServer is Inrun's HTTPS admission and conversion webhook surface.
// It owns the HTTPS server, all webhook handlers, registration, and the
// housekeeper that reconciles configurations with the API server.
//
// Created via NewWebhookServer and registered as a domain.Component in
// cmd/internal/runtime_construct.go. It starts after the HealthServer so that
// /ready is live before webhook registration runs.
type WebhookServer struct {
	name string

	// HTTPS server for all webhook endpoints.
	server *http.Server
	mux    *http.ServeMux

	// Lifecycle flags
	started atomic.Bool

	// Context derived from Start() — cancelled on Shutdown().
	ctx    context.Context
	cancel context.CancelFunc

	// TLS and port config
	tlsCert   string
	tlsKey    string
	httpsPort string

	// Webhook registration options passed to registration functions.
	hookReg WebhookRegistrationOptions

	// Declarative enablement flags resolved from Blueprint + config.
	webhooksEnabled  bool // admission (validation + mutation)
	convEnabled      bool // CRD conversion
	conversionWindow int  // rolling window size for conversion stats

	// Runtime protection state — set in Start() based on Blueprint.
	deletionProtection  atomic.Bool
	namespaceProtection atomic.Bool
	strictMode          atomic.Bool

	// Kubernetes client for webhook configuration registration.
	kubeClient kubernetes.Interface

	// Blueprint drives all conditional behavior.
	blueprint *blueprint.Blueprint

	// Registries — populated from Blueprint at construction time.
	conversionRegistry blueprint.ConversionRegistry
	admissionRegistry  blueprint.AdmissionRegistry

	// Stats — per-CRD maps keyed by GVR string ("group/version/resource").
	// Pre-populated from the Blueprint in NewWebhookServer so each CRD gets its
	// own counters. infraProtectionStats covers self-protection and Inrun
	// infra resources (Deployment, Service, etc.) that have no CRD GVR.
	admissionStats  map[string]*health.AdmissionStats
	conversionStats map[string]*health.ConversionStats
	protectionStats map[string]*health.DeletionProtectionStats
	namespaceStats  map[string]*health.NamespaceProtectionStats
	infraProtStats  *health.DeletionProtectionStats // webhook self + Inrun infra
	strictModeStats *health.DeletionProtectionStats // process-global; strict mode is not per-CRD
	webhookStats    *health.WebhookStats

	// Reverse-lookup tables built from the Blueprint for handlers that identify
	// the target CRD by name/kind rather than GVR.
	crdNameToGVRKey map[string]string // "plural.group" → "group/version/resource"
	gvkToGVRKey     map[string]string // "group/version/Kind" → "group/version/resource"

	// Deletion protection — set of CRD full names (plural.group) that are protected.
	protectedCRDNames map[string]struct{}

	// Namespace protection — map of CRD key (plural.group) → NamespaceRules.
	namespaceRuleMap map[string]*NamespaceRules

	// patchConversionCRD re-applies the caBundle on a conversion CRD.
	// Set by the caller via SetConversionCRDPatcher when conversion is enabled.
	patchConversionCRD ConversionCRDPatchFn

	// crdWatcher provides CRD watch capability for the conversion caBundle watcher.
	// Set by the caller via SetCRDWatcher. Nil when conversion is not enabled or
	// the caller does not provide one (safety ticker still covers the slow path).
	crdWatcher CRDWatcher

	// certMgr handles TLS secret cleanup on graceful shutdown.
	// Nil when the user provided explicit TLS_CERT/TLS_KEY env vars.
	certMgr certManagerIface

	// certSecretData holds the TLS bundle used at startup so the housekeeper can
	// restore the Secret if it is deleted (e.g. by a concurrent pod during rollout).
	// Nil when the user provided explicit TLS_CERT/TLS_KEY env vars.
	certSecretData      *certSecretBundle
	certSecretName      string
	certSecretNamespace string

	// Full config for namespace access during shutdown cleanup.
	config *config.Config

	// tokenReloader is called by the housekeeper when an Gateway API token secret
	// goes missing. Set via SetTokenReloader from the APIServer.
	tokenReloader func(ctx context.Context) error
}

// certSecretBundle carries the raw PEM bytes needed to restore the TLS Secret.
type certSecretBundle struct {
	certPEM []byte
	keyPEM  []byte
	caPEM   []byte
}

// NewWebhookServer constructs a WebhookServer and resolves all declarative
// webhook behavior from the Blueprint and Config. No I/O is performed — Start()
// activates servers, registration, and reconciliation.
func NewWebhookServer(kubeClient kubernetes.Interface, bp *blueprint.Blueprint, cfg *config.Config) *WebhookServer {
	admissionFailurePolicy := admissionv1FailurePolicyType(bp.WebhooksFailurePolicy())

	hookReg := WebhookRegistrationOptions{
		FailurePolicy:       admissionFailurePolicy,
		Port:                cfg.HTTPSPortInt32(),
		ServiceName:         bp.GatewayServiceName(),
		ServiceNamespace:    cfg.Cluster().Namespace(),
		TLSCertFile:         cfg.Security().Webhooks.TLSCert,
		InrunResourceLabels: labels.InrunResourceLabels(),
	}

	convWindow := bp.ConversionWindow()

	ws := &WebhookServer{
		name:               "webhook server",
		kubeClient:         kubeClient,
		blueprint:          bp,
		config:             cfg,
		httpsPort:          cfg.HTTPSPort(),
		tlsCert:            cfg.Security().Webhooks.TLSCert,
		tlsKey:             cfg.Security().Webhooks.TLSKey,
		hookReg:            hookReg,
		webhooksEnabled:    bp.IsAdmissionEnabled(),
		convEnabled:        bp.IsConversionEnabled(),
		conversionWindow:   convWindow,
		mux:                http.NewServeMux(),
		conversionRegistry: bp.ConversionRegistry(),
		admissionRegistry:  bp.AdmissionRegistry(),
		admissionStats:     make(map[string]*health.AdmissionStats),
		conversionStats:    make(map[string]*health.ConversionStats),
		protectionStats:    make(map[string]*health.DeletionProtectionStats),
		namespaceStats:     make(map[string]*health.NamespaceProtectionStats),
		infraProtStats:     health.NewDeletionProtectionStats(),
		webhookStats:       health.NewWebhookStats(),
		strictModeStats:    health.NewDeletionProtectionStats(),
		crdNameToGVRKey:    make(map[string]string),
		gvkToGVRKey:        make(map[string]string),
	}

	// Pre-populate per-CRD stat instances and reverse-lookup tables from the Blueprint.
	// Every CRD gets its own counters so the gateway /blueprint can return accurate
	// per-CRD breakdowns without mixing traffic across resources.
	// Must use Enabled() — raw Spec.CRDs entries have GroupVersionResource unset.
	for _, crd := range bp.Enabled() {
		gvr := crd.GVR()
		gvrKey := crdGVRKey(gvr.Group, gvr.Version, gvr.Resource)
		ws.admissionStats[gvrKey] = health.NewAdmissionStats(convWindow)
		ws.conversionStats[gvrKey] = health.NewConversionStats(convWindow)
		ws.protectionStats[gvrKey] = health.NewDeletionProtectionStats()
		ws.namespaceStats[gvrKey] = health.NewNamespaceProtectionStats()
		crdFullName := crd.APITypes.Plural + "." + crd.APITypes.Group
		ws.crdNameToGVRKey[crdFullName] = gvrKey
		// GVK key includes group+version so multi-version CRDs with the same kind
		// (e.g. cronjob-v1 and cronjob-v2) get distinct entries.
		ws.gvkToGVRKey[crdGVKKey(crd.APITypes.Group, crd.APITypes.Version, crd.APITypes.Kind)] = gvrKey
	}

	// Precompute deletion-protection CRD name set.
	if bp.IsDeletionProtectionEnabled() {
		ws.protectedCRDNames = bp.DeletionProtectedCRDNames()
	}

	// Precompute namespace rule map.
	if bp.IsNamespaceProtectionEnabled() {
		rawRules := bp.NamespaceProtectionRuleMap()
		if len(rawRules) > 0 {
			ws.namespaceRuleMap = make(map[string]*NamespaceRules, len(rawRules))
			for key, entry := range rawRules {
				rules := &NamespaceRules{
					Allowed:    make(map[string]struct{}, len(entry.Allowed)),
					Restricted: make(map[string]struct{}, len(entry.Restricted)),
				}
				for _, ns := range entry.Allowed {
					rules.Allowed[ns] = struct{}{}
				}
				for _, ns := range entry.Restricted {
					rules.Restricted[ns] = struct{}{}
				}
				ws.namespaceRuleMap[key] = rules
			}
		}
	}

	return ws
}

// crdGVRKey formats a GVR triple into a canonical string key used throughout
// the webhook stats maps: "group/version/resource" (or "version/resource" for core group).
func crdGVRKey(group, version, resource string) string {
	if group == "" {
		return version + "/" + resource
	}
	return group + "/" + version + "/" + resource
}

// crdGVKKey formats a GVK triple into the lookup key for gvkToGVRKey.
// Including version in the key prevents same-kind multi-version CRD entries
// (e.g. cronjob-v1 and cronjob-v2) from colliding on the kind name alone.
func crdGVKKey(group, version, kind string) string {
	if group == "" {
		return version + "/" + kind
	}
	return group + "/" + version + "/" + kind
}

// ── Per-CRD stat accessors ────────────────────────────────────────────────────
// These helpers return the per-CRD stats instance for a given GVR key.
// A nil-safe fallback is created on first miss (defensive; should not occur for
// CRDs declared in the Blueprint).

func (ws *WebhookServer) admissionStatsFor(gvrKey string) *health.AdmissionStats {
	if s, ok := ws.admissionStats[gvrKey]; ok {
		return s
	}
	s := health.NewAdmissionStats(ws.conversionWindow)
	ws.admissionStats[gvrKey] = s
	return s
}

func (ws *WebhookServer) conversionStatsFor(gvrKey string) *health.ConversionStats {
	if s, ok := ws.conversionStats[gvrKey]; ok {
		return s
	}
	s := health.NewConversionStats(ws.conversionWindow)
	ws.conversionStats[gvrKey] = s
	return s
}

func (ws *WebhookServer) protectionStatsFor(gvrKey string) *health.DeletionProtectionStats {
	if s, ok := ws.protectionStats[gvrKey]; ok {
		return s
	}
	s := health.NewDeletionProtectionStats()
	ws.protectionStats[gvrKey] = s
	return s
}

func (ws *WebhookServer) namespaceStatsFor(gvrKey string) *health.NamespaceProtectionStats {
	if s, ok := ws.namespaceStats[gvrKey]; ok {
		return s
	}
	s := health.NewNamespaceProtectionStats()
	ws.namespaceStats[gvrKey] = s
	return s
}

// SetCertManager provides the cert manager for TLS secret cleanup on graceful shutdown.
// Set only when Inrun generated self-signed certificates (certMgr == nil when the
// user provided explicit TLS_CERT/TLS_KEY).
func (ws *WebhookServer) SetCertManager(m certManagerIface) {
	ws.certMgr = m
}

// SetTokenReloader registers a callback that the housekeeper calls when an Gateway API
// token secret is found to be missing during reconciliation. The callback should
// recreate any missing secrets and reload the in-memory token set.
func (ws *WebhookServer) SetTokenReloader(fn func(ctx context.Context) error) {
	ws.tokenReloader = fn
}

// SetCertBundle stores the TLS bundle and Secret coordinates so the housekeeper can
// restore the Secret if it is deleted during a rollout. Only called when Inrun
// generated the certificates (not when the user provides TLS_CERT/TLS_KEY).
func (ws *WebhookServer) SetCertBundle(certPEM, keyPEM, caPEM []byte, secretName, namespace string) {
	ws.certSecretData = &certSecretBundle{
		certPEM: certPEM,
		keyPEM:  keyPEM,
		caPEM:   caPEM,
	}
	ws.certSecretName = secretName
	ws.certSecretNamespace = namespace
}

// Start activates the WebhookServer. It registers all declared webhook endpoints,
// launches the HTTPS server, performs best-effort webhook registration with the
// API server, and starts the reconciliation controller.
//
// Start is a no-op when no webhook capabilities are declared in the Blueprint or
// when the process is not running inside a Kubernetes cluster.
func (ws *WebhookServer) Start(ctx context.Context) error {
	ws.ctx, ws.cancel = context.WithCancel(ctx)
	ws.started.Store(true)

	bp := ws.blueprint

	// Resolve protection state from the Blueprint.
	if bp.IsDeletionProtectionEnabled() && bp.DeletionProtectionGVRs() != nil {
		ws.deletionProtection.Store(true)
	}
	if bp.IsNamespaceProtectionEnabled() && len(bp.NamespaceProtectionGVRs()) > 0 {
		ws.namespaceProtection.Store(true)
	}
	if bp.IsStrictModeEnabled() && kube.IsRunningInPod() {
		ws.strictMode.Store(true)
	}

	logger.Debug().
		Bool("deletionProtection", ws.deletionProtection.Load()).
		Bool("namespaceProtection", ws.namespaceProtection.Load()).
		Bool("conversion", ws.convEnabled).
		Bool("webhooks", ws.webhooksEnabled).
		Msg("webhook server configured")

	if !kube.IsRunningInPod() {
		logger.Debug().Msg("not running in cluster — skipping webhook HTTPS setup")
		return nil
	}

	startHTTPS := false
	admissionRuleExists := false

	// Register endpoints based on declared capabilities.
	if ws.deletionProtection.Load() {
		ws.mux.HandleFunc("/deletion-protection", ws.deletionProtectionHandler)
		startHTTPS = true
		logger.Info().Str("endpoint", "/deletion-protection").Msg("deletion protection endpoint registered")
	}
	if ws.namespaceProtection.Load() {
		ws.mux.HandleFunc("/namespace-protection", ws.namespaceProtectionHandler)
		startHTTPS = true
		logger.Info().Str("endpoint", "/namespace-protection").Msg("namespace protection endpoint registered")
	}
	if ws.strictMode.Load() {
		ws.mux.HandleFunc("/strict-mode-protection", ws.strictModeProtectionHandler)
		startHTTPS = true
		logger.Info().Str("endpoint", "/strict-mode-protection").Msg("strict mode protection endpoint registered")
	}
	if bp.HasConversionPaths() {
		ws.mux.HandleFunc("/convert", ws.conversionHandler)
		startHTTPS = true
		logger.Info().Str("endpoint", "/convert").Msg("conversion webhook endpoint registered")
	}
	if bp.HasValidationRules() {
		ws.mux.HandleFunc("/validate", ws.validationHandler)
		admissionRuleExists = true
		startHTTPS = true
		logger.Info().Str("endpoint", "/validate").Msg("validation endpoint registered")
	}
	if bp.HasMutationRules() {
		ws.mux.HandleFunc("/mutate", ws.mutationHandler)
		admissionRuleExists = true
		startHTTPS = true
		logger.Info().Str("endpoint", "/mutate").Msg("mutation endpoint registered")
	}

	// Start the housekeeper before the HTTPS gate — it manages token secrets and cert
	// rotation regardless of whether any webhook endpoints are active.
	if bp.IsHousekeeperEnabled() {
		if err := ws.housekeeper(ws.ctx); err != nil {
			logger.Error().Err(err).Msg("housekeeper failed to start")
		}
	}

	if !startHTTPS {
		logger.Debug().Msg("no webhook endpoints declared — HTTPS server not started")
		return nil
	}

	// Validate TLS prerequisites.
	if ws.tlsCert == "" {
		return fmt.Errorf("webhook server: TLS_CERT is required when webhook endpoints are active")
	}
	if ws.tlsKey == "" {
		return fmt.Errorf("webhook server: TLS_KEY is required when webhook endpoints are active")
	}

	httpsAddr := ws.httpsPort
	if !strings.HasPrefix(httpsAddr, ":") {
		httpsAddr = ":" + httpsAddr
	}

	ws.server = &http.Server{
		Addr:    httpsAddr,
		Handler: ws.mux,
	}

	go func() {
		logger.Info().
			Str("addr", httpsAddr).
			Str("cert", ws.tlsCert).
			Msg("webhook HTTPS server listening")
		if err := ws.server.ListenAndServeTLS(ws.tlsCert, ws.tlsKey); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Str("addr", httpsAddr).Msg("webhook HTTPS server error")
		}
	}()

	// Best-effort admission webhook registration.
	if admissionRuleExists && ws.kubeClient != nil && ws.admissionRegistry != nil {
		go func() {
			wctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := RegisterAdmissionWebhooks(wctx, ws.kubeClient, ws.admissionRegistry, ws.hookReg); err != nil {
				logger.Error().Err(err).Msg("admission webhook registration failed — RBAC may be missing")
			}
		}()
	}

	// Deletion protection webhook registration.
	if ws.deletionProtection.Load() && ws.kubeClient != nil {
		go func() {
			wctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			dpGVRs := bp.DeletionProtectionGVRs()
			if len(dpGVRs) == 0 {
				logger.Info().Msg("deletion protection: no GVRs — webhook not registered")
				return
			}

			caBundle, err := readCABundle(ws.hookReg.TLSCertFile)
			if err != nil {
				logger.Error().Err(err).Msg("deletion protection webhook: cannot read CA bundle")
				return
			}

			if err := registerDeletionProtectionWebhook(wctx, ws.kubeClient, dpGVRs, caBundle, ws.hookReg); err != nil {
				logger.Error().Err(err).Msg("deletion protection webhook registration failed")
			} else {
				logger.Info().
					Str("config", deletionProtectionWebhookConfigName).
					Int("rules", len(dpGVRs)).
					Int("protected", len(ws.blueprint.DeletionProtectedCRDNames())).
					Msg("deletion protection webhook registered")
			}
		}()
	}

	// Namespace protection webhook registration.
	if ws.namespaceProtection.Load() && ws.kubeClient != nil {
		go func() {
			wctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			npGVRs := bp.NamespaceProtectionGVRs()
			if len(npGVRs) == 0 {
				logger.Info().Msg("namespace protection: no GVRs — webhook not registered")
				return
			}

			caBundle, err := readCABundle(ws.hookReg.TLSCertFile)
			if err != nil {
				logger.Error().Err(err).Msg("namespace protection webhook: cannot read CA bundle")
				return
			}

			svcName := bp.GatewayServiceName()
			failurePolicy := bp.NamespaceProtectionFailurePolicy()
			if err := registerNamespaceProtectionWebhook(wctx, ws.kubeClient, npGVRs, caBundle, ws.hookReg, svcName, failurePolicy); err != nil {
				logger.Error().Err(err).Msg("namespace protection webhook registration failed")
			} else {
				logger.Info().
					Str("config", namespaceProtectionWebhookConfigName).
					Int("rules", len(npGVRs)).
					Msg("namespace protection webhook registered")
			}
		}()
	}

	// Strict mode webhook registration.
	if ws.strictMode.Load() && ws.kubeClient != nil {
		go func() {
			wctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			caBundle, err := readCABundle(ws.hookReg.TLSCertFile)
			if err != nil {
				logger.Error().Err(err).Msg("strict mode webhook: cannot read CA bundle")
				return
			}

			if err := registerStrictModeProtectionWebhook(wctx, ws.kubeClient, caBundle, ws.hookReg); err != nil {
				logger.Error().Err(err).Msg("strict mode protection webhook registration failed")
			} else {
				logger.Info().
					Str("config", strictModeProtectionWebhookConfigName).
					Msg("strict mode protection webhook registered")
			}
		}()
	}

	return nil
}

// Shutdown gracefully stops the HTTPS server and performs declarative cleanup
// of webhook configurations as declared in the Blueprint.
func (ws *WebhookServer) Shutdown(ctx context.Context) {
	if ws.cancel != nil {
		ws.cancel()
	}

	if ws.server == nil {
		return
	}

	if err := ws.server.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("webhook HTTPS server shutdown error")
	}

	bp := ws.blueprint

	// Cleanup admission webhook.
	cleanupOpts := WebhookCleanupOptions{}
	if bp.HasMutationRules() {
		cleanupOpts.mutating = bp.WebhookCleanupOnShutdown()
	}
	if bp.HasValidationRules() {
		cleanupOpts.validating = bp.WebhookCleanupOnShutdown()
	}
	if err := UnregisterAdmissionWebhooks(ctx, ws.kubeClient, cleanupOpts); err != nil {
		logger.Error().Err(err).Msg("webhook cleanup error")
	}

	// Cleanup deletion-protection webhook.
	if bp.IsDeletionProtectionEnabled() && bp.DeletionProtectionCleanupOnShutdown() && ws.kubeClient != nil {
		if err := cleanupValidatingWebhook(ctx, ws.kubeClient, deletionProtectionWebhookConfigName); err != nil {
			logger.Error().Err(err).Msg("deletion protection webhook cleanup error")
		} else {
			logger.Info().Str("config", deletionProtectionWebhookConfigName).Msg("deletion protection webhook removed")
		}
	}

	// Cleanup namespace-protection webhook.
	if bp.IsNamespaceProtectionEnabled() && bp.NamespaceProtectionCleanupOnShutdown() && ws.kubeClient != nil {
		if err := cleanupValidatingWebhook(ctx, ws.kubeClient, namespaceProtectionWebhookConfigName); err != nil {
			logger.Error().Err(err).Msg("namespace protection webhook cleanup error")
		} else {
			logger.Info().Str("config", namespaceProtectionWebhookConfigName).Msg("namespace protection webhook removed")
		}
	}

	// Cleanup strict-mode-protection webhook.
	// Strict mode cleanup mirrors deletion protection: only clean up when the
	// parent deletionProtection.cleanupOnShutdown is set, since strict mode is
	// a sub-feature of deletion protection.
	if bp.IsStrictModeEnabled() && bp.DeletionProtectionCleanupOnShutdown() && ws.kubeClient != nil {
		if err := cleanupValidatingWebhook(ctx, ws.kubeClient, strictModeProtectionWebhookConfigName); err != nil {
			logger.Error().Err(err).Msg("strict mode protection webhook cleanup error")
		} else {
			logger.Info().Str("config", strictModeProtectionWebhookConfigName).Msg("strict mode protection webhook removed")
		}
	}

	// Cleanup the TLS secret when auto-generated certs and cleanup is enabled.
	shouldCleanupTLS :=
		bp.WebhookCleanupOnShutdown() ||
			bp.NamespaceProtectionCleanupOnShutdown() ||
			bp.DeletionProtectionCleanupOnShutdown()

	if ws.certMgr != nil && shouldCleanupTLS && ws.config != nil {
		ns := ws.config.Cluster().Namespace()
		if err := ws.certMgr.DeleteCertificateAndSecret(ctx, ns, config.DefaultInternalTLSName()); err != nil {
			logger.Error().Err(err).Msg("tls secret cleanup error")
		} else {
			logger.Info().Str("namespace", ns).Msg("tls secret removed on shutdown")
		}
	}
}

// Name returns the component name.
func (ws *WebhookServer) Name() string { return ws.name }

// Started reports whether Start() has been called.
func (ws *WebhookServer) Started() bool { return ws.started.Load() }

// ── Stats getters — used by BuildGatewayBlueprintHandler ───────────────────────
// All stat getters are keyed by GVR string ("group/version/resource") so the
// gateway /blueprint handler can serve accurate per-CRD breakdowns.

// AdmissionStatsFor returns the admission stats for the CRD identified by gvrKey.
// Returns nil when no stats exist for that key (CRD has no admission webhooks).
func (ws *WebhookServer) AdmissionStatsFor(gvrKey string) *health.AdmissionStats {
	return ws.admissionStats[gvrKey]
}

// ConversionStatsFor returns the conversion stats for the CRD identified by gvrKey.
func (ws *WebhookServer) ConversionStatsFor(gvrKey string) *health.ConversionStats {
	return ws.conversionStats[gvrKey]
}

// ProtectionStatsFor returns the deletion-protection stats for the CRD identified by gvrKey.
func (ws *WebhookServer) ProtectionStatsFor(gvrKey string) *health.DeletionProtectionStats {
	return ws.protectionStats[gvrKey]
}

// NamespaceStatsFor returns the namespace-protection stats for the CRD identified by gvrKey.
func (ws *WebhookServer) NamespaceStatsFor(gvrKey string) *health.NamespaceProtectionStats {
	return ws.namespaceStats[gvrKey]
}

// InfraProtectionStats returns the process-level deletion-protection stats that
// cover the webhook configuration itself and Inrun infra resources (Deployment,
// Service, etc.) — events not attributable to a specific CRD GVR.
func (ws *WebhookServer) InfraProtectionStats() *health.DeletionProtectionStats {
	return ws.infraProtStats
}

// HousekeeperStats returns the housekeeper reconciliation statistics.
func (ws *WebhookServer) HousekeeperStats() *health.WebhookStats { return ws.webhookStats }

// StrictModeStats returns the strict-mode-protection statistics (process-global).
func (ws *WebhookServer) StrictModeStats() *health.DeletionProtectionStats {
	return ws.strictModeStats
}

// GVRKey formats group/version/resource into the canonical stats key.
// Exported so the gateway handler can build keys from crd.GVR() without
// duplicating the formatting logic.
func GVRKey(group, version, resource string) string {
	return crdGVRKey(group, version, resource)
}
