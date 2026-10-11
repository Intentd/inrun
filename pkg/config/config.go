package config

import (
	"os"
	"strconv"
	"time"

	"github.com/intentd/inrun/pkg/utils"
	"github.com/joho/godotenv"
)

func Init(filenames ...string) (*Config, error) {
	// load .env files for tesing...
	_ = godotenv.Load(filenames...)
	ns := resolveNamespace()

	kfg := &Config{
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
		// ── Unified security configuration ───────────────────────────────────
		// ENV vars populate SecurityConfig as defaults.
		// Catalog YAML values are merged on top in BuildRuntimeCatalog.
		//
		// ENV → SecurityConfig mapping:
		//   ENABLE_DELETION_PROTECTION  → security.DeletionProtection.Enabled
		//   DELETION_PROTECTION_POLICY  → security.DeletionProtection.FailurePolicy
		//   ENABLE_ADMISSION_WEBHOOK    → security.Webhooks.Admission.Enabled
		//   ENABLE_CONVERSION           → security.Conversion.Enabled
		//   WEBHOOK_FAILURE_POLICY      → security.Webhooks.FailurePolicy
		//   INRUN_SERVICE_NAME       → security.Webhooks.ServiceName
		//                               → security.DeletionProtection.ServiceName
		//   CONVERSION_WINDOW           → security.Conversion.ConversionWindow
		//   TLS_CERT / TLS_KEY          → security.Webhooks.TLSCert / TLSKey (initial
		//                                 values; overwritten by ensureSecurity() when
		//                                 Inrun generates its own certificates)
		security: func() SecurityConfig {
			var s SecurityConfig
			runtimeSvc := GetStrEnv("INRUN_SERVICE_NAME", "inrun-runtime")
			gatewaySvc := GetStrEnv("INRUN_GATEWAY_SERVICE_NAME", "inrun-gateway")
			s.ServiceName.Runtime = runtimeSvc
			s.ServiceName.Gateway = gatewaySvc

			s.DeletionProtection.Enabled = GetBoolEnv("ENABLE_DELETION_PROTECTION", false)
			s.DeletionProtection.FailurePolicy = GetStrEnv("DELETION_PROTECTION_POLICY", "Fail")
			s.Webhooks.Admission.Enabled = GetBoolEnv("ENABLE_ADMISSION_WEBHOOK", false)
			s.Conversion.Enabled = GetBoolEnv("ENABLE_CONVERSION", false)
			s.Webhooks.FailurePolicy = GetStrEnv("WEBHOOK_FAILURE_POLICY", "Ignore")

			s.DeletionProtection.ServiceName = gatewaySvc
			s.Webhooks.ServiceName = gatewaySvc
			s.NamespaceProtection.ServiceName = gatewaySvc

			s.Conversion.ConversionWindow = GetIntEnv("CONVERSION_WINDOW", 100)
			s.Webhooks.TLSCert = GetStrEnv("TLS_CERT", "")
			s.Webhooks.TLSKey = GetStrEnv("TLS_KEY", "")
			s.Webhooks.Housekeeper.Enabled = GetBoolEnv("ENABLE_HOUSEKEEPER", true)
			s.Webhooks.Housekeeper.SyncInterval = GetDurEnvSeconds("HOUSEKEEPER_SYNC_INTERVAL", 30)
			s.NamespaceProtection.Enabled = GetBoolEnv("ENABLE_NAMESPACE_PROTECTION", false)
			s.NamespaceProtection.FailurePolicy = GetStrEnv("NAMESPACE_PROTECTION_FAILURE_POLICY", "Fail")
			s.NamespaceProtection.CleanupOnShutdown = GetBoolEnv("NAMESPACE_PROTECTION_CLEANUP_ON_SHUTDOWN", false)
			s.CertManager.AutoRotate = GetBoolEnv("TLS_AUTO_ROTATE", true)
			s.CertManager.RotationThreshold = GetStrEnv("TLS_ROTATION_THRESHOLD", "30d")
			s.CertManager.ValidFor = GetStrEnv("TLS_VALID_FOR", "1y")
			return s
		}(),

		registry: registryConfig{
			RegistryURL: GetStrEnv("INRUN_REGISTRY", ""),
		},
		healthServer: healthServer{
			port:         GetStrEnv("INRUN_PORT", "8080"),
			readTimeout:  GetDurEnvSeconds("SRV_READ_TIMEOUT", 5),
			writeTimeout: GetDurEnvSeconds("SRV_WRITE_TIMEOUT", 20),
		},
		leader: leaderElection{
			namespace:     ns,
			leaseDuration: GetDurEnvSeconds("LEASE_DURATION", 60),
			renewDeadline: GetDurEnvSeconds("RENEW_DEADLINE", 40),
			retryPeriod:   GetDurEnvSeconds("RETRY_PERIOD", 10),
		},
		catalog: catalogConfig{
			defaultQueueDepth:       GetIntEnv("QUEUE_DEPTH", 100),
			defaultFailureThreshold: GetIntEnv("FAILURE_THRESHOLD", 5),
			paths:                   GetStrSliceEnv("CATALOG_PATH", []string{}),
			defaultResync:           GetDurEnvSeconds("DEFAULT_RESYNC", 15),
			defaultWorkers:          GetIntEnv("DEFAULT_WORKERS", 3),
			shutdownTimeout:         GetDurEnvSeconds("SHUTDOWN_TIMEOUT", 30),
			shutdownGracePeriod:     GetDurEnvSeconds("SHUTDOWN_GRACE_PERIOD", 60),
			gatewayEndpoint:         GetStrEnv("INRUN_GATEWAY_ENDPOINT", ""),
		},
	}

	// normalize environment
	kfg.normalizeEnvironment()

	// validate struct
	if err := Validate().Struct(kfg); err != nil {
		return nil, err
	}

	return kfg, nil
}

// -----------------------------------------------------------------------------

// SetInstance sets the active Inrun instance name (runtime or gateway)
// on the Config. This controls which service name is used when resolving
// endpoints and wiring.
func (k *Config) SetInstance(instance Instance) {
	k.inrun.instance = instance
}

// IsRuntimeInstance reports whether the active instance is the internal
// Inrun runtime service.
func (k *Config) IsRuntimeInstance() bool {
	return k.inrun.instance == InstanceRuntime
}

// IsGatewayInstance reports whether the active instance is the external
// Inrun gateway service.
func (k *Config) IsGatewayInstance() bool {
	return k.inrun.instance == InstanceGateway
}

// GetStrEnv returns the string value of an env
func GetStrEnv(key, def string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return def
}

// GetStrSliceEnv returns the slice value of an env
func GetStrSliceEnv(key string, def []string) []string {
	if val, ok := os.LookupEnv(key); ok {
		return []string{val}
	}
	return def
}

// GetBoolEnv returns the boolean value of an env
func GetBoolEnv(key string, def bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if valBool, err := strconv.ParseBool(val); err == nil {
			return valBool
		}
	}
	return def
}

// GetDurEnvSeconds returns the time.duration value of an env
func GetDurEnvSeconds(key string, def int) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if valInt, err := strconv.Atoi(val); err == nil {
			return time.Duration(valInt) * time.Second
		}
	}
	return time.Duration(def) * time.Second
}

// GetIntEnv returns the int value of an env
func GetIntEnv(key string, def int) int {
	if val, ok := os.LookupEnv(key); ok {
		if valInt, err := strconv.Atoi(val); err == nil {
			return valInt
		}
	}
	return def
}

// resolveNamespace resolves the namespace for use by all internal inrun resources
func resolveNamespace() string {
	// Resolve namespace
	if os.Getenv("INRUN_NAMESPACE") == "" {
		// Set namespace to default if running outside a pod
		// This is helpful for quick testing using an 'always available' namespace
		if !utils.IsRunningInPod() {
			return "default"
		}
	}

	return GetStrEnv("INRUN_NAMESPACE", "inrun-system")
}
