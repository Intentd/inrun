package config

import (
	"time"

	"github.com/inrundev/inrun/pkg/labels"
)

type Config struct {
	inrun        inrunConfig
	cluster      clusterConfig
	leader       leaderElection
	healthServer healthServer
	catalog      catalogConfig
	security     SecurityConfig
	registry     registryConfig
}

type inrunConfig struct {
	name        string   `validate:"required"`
	instance    Instance // runtime or gateway
	shortName   string
	environment string
	logLevel    string
}

type healthServer struct {
	port         string
	readTimeout  time.Duration
	writeTimeout time.Duration
}

type clusterConfig struct {
	kubeconfigPath string
	masterURL      string
	name           string
	namespace      string `validate:"required"`
}

type registryConfig struct {
	RegistryURL string
}

// SecurityConfig is the unified security configuration populated from ENV vars
// at Init() time. Catalog YAML values are merged on top via the Catalog
// loader, so this represents the ENV-level defaults.
//
// Precedence: Catalog YAML > SecurityConfig (ENV) > hard default.
type SecurityConfig struct {
	ServiceName struct {
		Runtime string
		Gateway string
	}

	DeletionProtection struct {
		Enabled           bool
		CleanupOnShutdown bool
		ServiceName       string
		FailurePolicy     string
	}
	Webhooks struct {
		Admission struct {
			Enabled bool
		}
		CleanupOnShutdown bool
		FailurePolicy     string
		ServiceName       string
		// TLS paths — shared with deletion protection, admission, and conversion.
		// Set by ensureSecurity() after cert generation/loading.
		TLSCert string
		TLSKey  string

		Housekeeper struct {
			Enabled      bool
			SyncInterval time.Duration
		}
	}
	// Conversion is separate from admission webhooks — conversion has its own
	// /convert endpoint, window stats, and CRD patch logic.
	Conversion struct {
		Enabled bool
		// ConversionWindow is the rolling window size for latency/throughput stats.
		ConversionWindow  int
		CleanupOnShutdown bool // TODO
	}

	// NamespaceProtection controls the optional validating webhook that prevents
	// Inrun-managed CRs from being created or updated in forbidden namespaces.
	//
	// This is an admission-time safeguard only. If disabled, namespace rules are
	// not enforced at apply time. If enabled, the webhook blocks CRs whose target
	// namespace violates the CRD’s declared allowedNamespaces or restrictedNamespaces.
	//
	// The webhook is managed by the housekeeper and will be recreated if
	// deleted, ensuring continuous enforcement when enabled.
	//
	// Precedence: Catalog YAML > SecurityConfig (ENV) > hard default.
	NamespaceProtection struct {
		Enabled           bool
		FailurePolicy     string
		ServiceName       string
		CleanupOnShutdown bool
	}

	// CertManager controls the lifecycle of Inrun’s auto-generated TLS certificate.
	// Only applies when certificates are auto-generated (TLS_CERT/TLS_KEY not set).
	//
	// Precedence: Catalog YAML > SecurityConfig (ENV) > hard default.
	CertManager struct {
		// AutoRotate enables pre-emptive certificate rotation before expiry.
		// Default: true. Set TLS_AUTO_ROTATE=false to opt out.
		AutoRotate bool
		// RotationThreshold is how far before expiry Inrun rotates.
		// Parsed from TLS_ROTATION_THRESHOLD env (e.g. "30d"). Default: "30d".
		RotationThreshold string
		// ValidFor is the default certificate validity duration.
		// Parsed from TLS_ROTATE_AFTER env (e.g. "30d"). Default: "1y".
		ValidFor string
	}
}

type catalogConfig struct {
	paths                   []string // Comma separated Paths to CRD catalog YAML file
	defaultQueueDepth       int
	defaultFailureThreshold int `validate:"required"`
	defaultResync           time.Duration
	defaultWorkers          int
	shutdownTimeout         time.Duration
	shutdownGracePeriod     time.Duration
	// gatewayEndpoint is advertised in the runtime /catalog response so the
	// console can locate the companion gateway and merge stats.
	// Populated from INRUN_GATEWAY_ENDPOINT; empty when no gateway is configured.
	gatewayEndpoint string
}

type leaderElection struct {
	namespace     string
	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
}

// Methods

// NewDefaultConfig returns a Config populated from environment variables with
// sensible defaults. Used by CLI commands that do not go through the full
// Init() path (e.g. inrun validate, inrun template, inrun simulate). All fields
// follow the same GetStrEnv/GetIntEnv/GetDurEnvSeconds pattern as Init().
func NewDefaultConfig() *Config {
	ns := resolveNamespace()
	return &Config{
		inrun: inrunConfig{
			name:        Name,
			shortName:   CLI,
			environment: GetStrEnv("INRUN_ENV", "development"),
			logLevel:    GetStrEnv("LOG_LEVEL", "info"),
		},
		cluster: clusterConfig{
			kubeconfigPath: GetStrEnv("KUBECONFIG", ""),
			masterURL:      GetStrEnv("MASTER_URL", ""),
			name:           GetStrEnv("CLUSTER_NAME", ""),
			namespace:      ns,
		},
		leader: leaderElection{
			namespace:     ns,
			leaseDuration: GetDurEnvSeconds("LEASE_DURATION", 60),
			renewDeadline: GetDurEnvSeconds("RENEW_DEADLINE", 40),
			retryPeriod:   GetDurEnvSeconds("RETRY_PERIOD", 10),
		},
		healthServer: healthServer{
			port:         GetStrEnv("INRUN_PORT", "8080"),
			readTimeout:  GetDurEnvSeconds("SRV_READ_TIMEOUT", 5),
			writeTimeout: GetDurEnvSeconds("SRV_WRITE_TIMEOUT", 20),
		},
		catalog: catalogConfig{
			paths:                   GetStrSliceEnv("CATALOG_PATH", []string{"catalog.yaml"}),
			defaultWorkers:          GetIntEnv("DEFAULT_WORKERS", 3),
			defaultResync:           GetDurEnvSeconds("DEFAULT_RESYNC", 15),
			defaultQueueDepth:       GetIntEnv("QUEUE_DEPTH", 100),
			defaultFailureThreshold: GetIntEnv("FAILURE_THRESHOLD", 5),
		},
	}
}

// IsDev returns true for development environment
func (k *Config) IsDev() bool {
	return k.Inrun().environment == "devlopment"
}

// IsDev returns true for staging environment
func (k *Config) IsStaging() bool {
	return k.Inrun().environment == "staging"
}

// IsDev returns true for production environment
func (c *Config) IsProduction() bool {
	return c.Inrun().environment == "production"
}

// Health returns health configurations
func (k *Config) Health() *healthServer {
	return &k.healthServer
}

// Inrun returns Inrun Configurations
func (k *Config) Inrun() *inrunConfig {
	return &k.inrun
}

// Runtime service name
func (k *Config) RuntimeServiceName() string {
	return k.security.ServiceName.Runtime
}

// Gateway service name
func (k *Config) GatewayServiceName() string {
	return k.security.ServiceName.Gateway
}

// Running instance returns the current running inrun instance
func (k *Config) RunningInstance() string {
	return k.inrun.instance.String()
}

// Cluster returns cluster Configurations
func (k *Config) Cluster() *clusterConfig {
	return &k.cluster
}

// Leader returns leader Configurations
func (c *Config) Leader() *leaderElection {
	return &c.leader
}

// Catalog returns catalog Configurations
func (k *Config) Catalog() *catalogConfig {
	return &k.catalog
}

// Finalizers return a list of default finalizers
func (k *Config) Finalizers() []string {
	return []string{labels.FinalizerInrun}
}

// Security returns the unified security configuration.
// This is the primary accessor for all security-related settings.
func (k *Config) Security() *SecurityConfig {
	return &k.security
}

// RegistryConfig returns registry configuration.
func (k *Config) RegistryConfig() *registryConfig {
	return &k.registry
}

// ConversionEnabled reports whether the conversion webhook is enabled.
// Reads from SecurityConfig (populated from ENV at Init).
func (k *Config) ConversionEnabled() bool {
	return k.security.Conversion.Enabled
}

// AdmissionEnabled reports whether admission webhooks are enabled.
// Reads from SecurityConfig (populated from ENV at Init).
func (k *Config) AdmissionEnabled() bool {
	return k.security.Webhooks.Admission.Enabled
}

// GatewayEndpoint returns the companion gateway URL advertised to the console.
// Empty string when no gateway is configured (e.g. runtime-only deployment).
func (k *Config) GatewayEndpoint() string {
	return k.catalog.gatewayEndpoint
}

// SetGatewayEndpoint advertises url as the companion gateway unless
// INRUN_GATEWAY_ENDPOINT already set one. The CLI calls it when it starts a
// local gateway next to the runtime.
func (k *Config) SetGatewayEndpoint(url string) {
	if k.catalog.gatewayEndpoint == "" {
		k.catalog.gatewayEndpoint = url
	}
}

// HTTPSPort returns the HTTPS port string (e.g. ":8443") used by the webhook server.
func (k *Config) HTTPSPort() string {
	return httpsPort
}

// HTTPSPortInt32 returns the HTTPS port as int32 (8443) used in webhook client configs.
func (k *Config) HTTPSPortInt32() int32 {
	return httpsPortInt32
}

// DefaultInternalTLSName returns the default name for Inrun's internal TLS secret.
func DefaultInternalTLSName() string {
	return defaultInternalTLSSecretName
}

// DefaultWorkloadSecretName returns the base name used for generated workload secrets.
// The caller appends the CR's name to form the final secret name.
func DefaultWorkloadSecretName() string {
	return defaultWorkloadSecretName
}
