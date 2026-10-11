// cmd/internal/gateway.go
//
// Gateway startup — handles TLS/security, admission/conversion webhooks,
// and the Serve layer (Gateway API + intake webhooks). No reconcilers, no
// informer factory, no leader election (webhook servers are stateless and
// can run as multiple replicas).

//go:build gateway

package internal

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/pipeline"
	"github.com/intentd/inrun/pkg/config"
	apigateway "github.com/intentd/inrun/pkg/gateway/api"
	"github.com/intentd/inrun/pkg/gateway/certmanager"
	gwhandlers "github.com/intentd/inrun/pkg/gateway/handlers"
	"github.com/intentd/inrun/pkg/gateway/webhook"
	"github.com/intentd/inrun/pkg/health"
	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/process"
	"github.com/intentd/inrun/pkg/utils"
)

// RunGateway starts the production gateway — TLS, WebhookServer
// (admission/conversion), and the Serve layer (Gateway API + intake webhooks).
// No leader election — the gateway is stateless and supports multiple replicas.
func RunGateway(kfg *config.Config, m *merger.Merger, ctx context.Context) {

	if !utils.IsRunningInPod() {
		fmt.Println("inrun: inrun gate only runs inside a Kubernetes pod. Use 'inrun gate run' for local development.")
		os.Exit(1)
	}

	// ── 1a. Instance ────────────────────────────────────────────────────────────
	kfg.SetInstance(config.Gateway())

	// ── 1b. Catalog ────────────────────────────────────────────────────────────
	// Needed to know which CRDs require webhooks.
	kat := pipeline.NewCatalog(kfg, m)

	if registryURL := kfg.RegistryConfig().RegistryURL; registryURL != "" {
		m.SetRegistryURL(registryURL)
		logger.Info().Str("registry", registryURL).Msg("registry URL configured from INRUN_REGISTRY")
	}

	// ── 2. Scheme ─────────────────────────────────────────────────────────────
	scheme, err := catalog.NewSchemeRegistry(kat)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to build scheme registry")
	}

	// ── 3. Kubeclient ─────────────────────────────────────────────────────────
	kube := kubeclient.NewKubeclient(kfg, scheme)

	if err := kube.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to start kubeclient")
	}

	// ── 4. Security ───────────────────────────────────────────────────────────
	// TLS cert management + CRD conversion webhook patching.
	// Only runs in-cluster; the guard at the top of this function ensures we
	// never reach this point outside a pod.
	var certMgr certmanager.Manager
	var tlsCert, tlsKey string
	var secErr error
	var tlsBundle *certmanager.TLSBundle
	tlsCert, tlsKey, certMgr, tlsBundle, secErr = ensureSecurity(ctx, kfg, kat, kube)
	if secErr != nil {
		logger.Fatal().Err(secErr).Msg("security setup failed")
	}

	if tlsCert != "" {
		logger.Debug().
			Str("cert_file", tlsCert).
			Str("cert_key", tlsKey).
			Msg("passing generated TLS cert to webhook server")
		kfg.Security().Webhooks.TLSCert = tlsCert
		kfg.Security().Webhooks.TLSKey = tlsKey
	}

	// ── 5. HealthServer — HTTP (probes + metrics + /catalog API) ────────────────
	hs := health.NewHealthServer(kfg)

	// ── 6. WebhookServer ──────────────────────────────────────────────────────
	ws := webhook.NewWebhookServer(kube.Clientset(), kat, kfg)
	if certMgr != nil {
		ws.SetCertManager(certMgr)
	}
	if tlsBundle != nil {
		ws.SetCertBundle(tlsBundle.CertPEM, tlsBundle.KeyPEM, tlsBundle.CACertPEM,
			certmanager.DefaultTLSSecretName, kfg.Cluster().Namespace())
	}
	// Wire housekeeper infrastructure reconcilers — keeps namespace labels and
	// CRD conversion caBundles correct throughout the deployment lifecycle.
	WireWebhookHousekeeperInfra(ws, kube, kat, kfg)

	// ── 7. /catalog routes — gateway serves its own stats surface ────────────
	// The console discovers this endpoint via the "gatewayEndpoint" field
	// in the runtime /catalog response and merges per-CRD stats by GVR key.
	hs.Register("/catalog", gwhandlers.BuildGatewayCatalogHandler(kat, ws))

	for _, crd := range kat.Enabled() {
		crdName := strings.ToLower(crd.Name)
		gvr := crd.GVR()
		gvrKey := webhook.GVRKey(gvr.Group, gvr.Version, gvr.Resource)
		hs.Register(
			"/catalog/"+crdName,
			gwhandlers.BuildGatewayCRDHandler(crd.Name, crd.GVKString(), gvr.String(), gvrKey, ws, kat),
		)
	}
	logger.Debug().Msg("gateway /catalog routes registered")

	// ── 7b. Gateway API ────────────────────────────────────────────────────────
	// Registers POST /api/v1/apply, GET/DELETE /api/v1/resources/, GET /api/v1/schema/
	// only when gateway.api.enabled: true in the Catalog and at least one
	// CRD has serve.enabled: true.

	// Build the cluster registry once — shared by the API server and intake server.
	clusters, clustersErr := apigateway.BuildClusterRegistry(ctx, kat, kube, kfg.Cluster().Namespace())
	if clustersErr != nil {
		logger.Fatal().Err(clustersErr).Msg("gateway cluster registry setup failed")
	}

	api, apiErr := apigateway.NewAPIServer(ctx, kat, kube, clusters, kfg.Cluster().Namespace())
	if apiErr != nil {
		logger.Fatal().Err(apiErr).Msg("gateway API setup failed")
	}

	if api != nil {
		api.Register(hs)
		ws.SetTokenReloader(api.ReloadTokens)
	}

	// ── 8. Component list ─────────────────────────────────────────────────────
	components := []domain.Component{
		hs,   // 1. HTTP server — /ready, /livez probes
		ws,   // 2. HTTPS webhook server — /validate, /mutate, /convert
		kube, // 3. REST clients — already started, managed for Stop()
	}

	// ── 8. Inrun ───────────────────────────────────────────────────────────
	o := process.New(
		kfg.RunningInstance(),
		kfg.Catalog().ShutdownGracePeriod(),
		kfg.Inrun().LogLevel(),
	)
	o.Register(components)

	// ── Start and wait (no leader election) ──────────────────────────────────
	go func() {
		if err := o.Start(ctx); err != nil {
			logger.Fatal().AnErr("gateway startup error", err)
			utils.Exit(err)
		}
	}()

	logger.Info().Msg("gateway started — serving webhooks")

	o.Wait()
}
