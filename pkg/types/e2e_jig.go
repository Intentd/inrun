package types

import "github.com/intentd/jig"

// The e2e spec's environment, expectations, kubectl DSL, side effects,
// imports and ecosystem live in jig (github.com/intentd/jig); these names
// keep Inrun's e2e types reading as before.
type (
	Provision                = jig.Provision
	ProvisionApplyEntry      = jig.ApplyEntry
	ProvisionHelmInstall     = jig.HelmInstall
	ProvisionWait            = jig.Wait
	ProvisionApplyEntries    = jig.ApplyEntries
	ProvisionHelmInstalls    = jig.HelmInstalls
	ProvisionWaits           = jig.Waits
	E2EEnvironment           = jig.Environment
	E2EImport                = jig.Import
	E2EOnFailure             = jig.OnFailure
	E2EFinally               = jig.Finally
	E2ESideEffects           = jig.SideEffects
	ClusterProvider          = jig.ClusterProvider
	E2ECluster               = jig.Cluster
	E2EAfter                 = jig.After
	E2EExpectation           = jig.Expectation
	E2EExpectationScope      = jig.ExpectationScope
	E2EResourceCheck         = jig.ResourceCheck
	E2ECommand               = jig.Command
	E2EKubectl               = jig.Kubectl
	E2EExternalCheck         = jig.ExternalCheck
	E2EExternalChecks        = jig.ExternalChecks
	E2EKubectlGet            = jig.KubectlGet
	E2EKubectlLogs           = jig.KubectlLogs
	E2ELogsPods              = jig.LogsPods
	E2EKubectlDescribe       = jig.KubectlDescribe
	E2EKubectlExec           = jig.KubectlExec
	E2EKubectlApply          = jig.KubectlApply
	E2EKubectlDelete         = jig.KubectlDelete
	E2EKubectlPatch          = jig.KubectlPatch
	E2EKubectlEvents         = jig.KubectlEvents
	E2EKubectlAuth           = jig.KubectlAuth
	E2EKubectlCp             = jig.KubectlCp
	E2EKubectlTop            = jig.KubectlTop
	E2EKubectlLeaderElection = jig.KubectlLeaderElection
	E2EKubectlPortForward    = jig.KubectlPortForward
	E2EKubectlRestart        = jig.KubectlRestart
	E2EKubectlScale          = jig.KubectlScale
	E2EExpectations          = jig.Expectations
	DataService              = jig.DataService
	Ecosystem                = jig.Ecosystem
	EcosystemPostgres        = jig.EcosystemPostgres
	EcosystemMariadb         = jig.EcosystemMariadb
	EcosystemRabbitmq        = jig.EcosystemRabbitmq
	EcosystemRedis           = jig.EcosystemRedis
	EcosystemKafka           = jig.EcosystemKafka
	EcosystemSecrets         = jig.EcosystemSecrets
	EcosystemExport          = jig.EcosystemExport
	EcosystemDefine          = jig.EcosystemDefine
	E2EExpectInclude         = jig.ExpectInclude
	E2EExternalInclude       = jig.ExternalInclude
	E2EImagesInclude         = jig.ImagesInclude
	E2EApplyInclude          = jig.ApplyInclude
	E2EHelmInclude           = jig.HelmInclude
)

var (
	ProviderKind        = jig.ProviderKind
	ProviderNone        = jig.ProviderNone
	E2ELogsPodsAny      = jig.LogsPodsAny
	E2ELogsPodsAll      = jig.LogsPodsAll
	ValidLogsPodsValues = jig.ValidLogsPodsValues
)

// ValidClusterProviders returns the supported providers, in display order.
func ValidClusterProviders() []string { return jig.ValidClusterProviders() }

// IsE2EExpectIncludeDoc reports whether the file is a valid expect include.
func IsE2EExpectIncludeDoc(path string) (bool, error) { return jig.IsExpectIncludeDoc(path) }

// IsE2EExternalIncludeDoc reports whether the file is a valid external check include.
func IsE2EExternalIncludeDoc(path string) (bool, error) { return jig.IsExternalIncludeDoc(path) }
