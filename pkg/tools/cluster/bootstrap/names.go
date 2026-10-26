package bootstrap

const (
	DefaultSAName      = "inrun-gateway"
	DefaultSANamespace = "kube-system"
)

// KnownVerbs is the set of valid Kubernetes RBAC verbs.
var KnownVerbs = map[string]bool{
	"get":              true,
	"list":             true,
	"watch":            true,
	"create":           true,
	"update":           true,
	"patch":            true,
	"delete":           true,
	"deletecollection": true,
	"use":              true,
	"bind":             true,
	"escalate":         true,
	"impersonate":      true,
	"approve":          true,
	"sign":             true,
	"*":                true,
}

// SAName returns the ServiceAccount name for this entry.
// Falls back to DefaultSAName when sa-name is not set.
func SAName(e ClusterEntry) string {
	if e.SAName != "" {
		return e.SAName
	}
	return DefaultSAName
}

// ClusterRoleName returns the ClusterRole name — always matches the SA name.
func ClusterRoleName(e ClusterEntry) string { return SAName(e) }

// CRBName returns the ClusterRoleBinding name — always matches the SA name.
func CRBName(e ClusterEntry) string { return SAName(e) }

// TokenSecretName returns the name of the long-lived token Secret on the target cluster.
func TokenSecretName(e ClusterEntry) string { return SAName(e) + "-token" }

// GatewaySecretName returns the name of the credential Secret written on the gateway cluster.
func GatewaySecretName(e ClusterEntry) string { return "inrun-" + e.Name }

// Labels returns the standard labels applied to all bootstrap-created resources.
func Labels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "inrun",
		"inrun.dev/role":               "gateway-cluster-access",
	}
}
