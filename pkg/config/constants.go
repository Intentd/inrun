package config

const (
	// Product names
	Name          = "Inrun"
	CLI           = "inrun"
	InrunOperator = "inrun-operator"

	// SSA field managers
	FieldManagerRuntime = "inrun-runtime"
	FieldManagerGateway = "inrun-gateway"

	// Environment
	DevShort     = "dev"
	StagingShort = "uat"
	Live         = "live"
	ProdShort    = "prod"
	Development  = "development"
	Staging      = "staging"
	Production   = "production"

	// Modes
	DynamicMode = "dynamic"
	TypedMode   = "typed"

	// Kind
	kindBlueprint = "Blueprint"
	kindLeader    = "Leader"
	kindStack     = "Stack"
	kindModule    = "Module"
	kindE2E       = "E2E"
	kindSimulate  = "Simulate"

	// HTTPS Port
	httpsPort      = ":8443"
	httpsPortInt32 = 8443

	// Secrets
	defaultInternalTLSSecretName = "inrun-internal-tls"
	defaultWorkloadSecretName    = "inrun-tls"
)

// Instance identifiers used by Inrun to distinguish between the internal
// runtime service and gateway service.
type Instance string

const (
	InstanceRuntime Instance = "runtime"
	InstanceGateway Instance = "gateway"
)

var (
	apiVersions = []string{
		"inrun.dev/v1",
	}
)
