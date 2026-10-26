package generate

import "time"

// BundleOptions controls which components are included in a generated bundle.
// Each flag defaults to true; pass false to exclude that component's ServiceAccount,
// ClusterRole, and ClusterRoleBinding from the output.
// IncludeConfigMap follows: included when IncludeRuntime || IncludeGateway.
type BundleOptions struct {
	IncludeRuntime bool
	IncludeGateway bool
	IncludeConsole bool
}

func DefaultBundleOptions() BundleOptions {
	return BundleOptions{
		IncludeRuntime: true,
		IncludeGateway: true,
		IncludeConsole: true,
	}
}

type registryTemplateData struct {
	Timestamp string
	Imports   []importEntry
	Entries   []registryEntry
	//	SchemeEntries    []registryEntry
	HookEntries        []hookEntry
	RecEntries         []reconcilerEntry
	TargetHookEntries  []targetHookEntry
	TargetRecEntries   []targetRecEntry
	NeedsRecImports    bool // true when RecEntries or TargetRecEntries is non-empty
	NeedsHookImports   bool // true when HookEntries or TargetHookEntries is non-empty
	NeedsSchemeImports bool // true when Entries is non-empty (metav1.AddToGroupVersion needed)
}

type importEntry struct {
	Alias    string
	Location string
}

type registryEntry struct {
	Alias   string
	Object  string
	List    string
	Group   string
	Version string
	Kind    string
}

type hookEntry struct {
	Alias    string
	Function string
	Group    string
	Version  string
	Kind     string
}

type reconcilerEntry struct {
	Alias    string
	Function string
	Group    string
	Version  string
	Kind     string
}

// targetHookEntry represents one TargetHookRegistry assignment in the generated file.
// TargetName is the serve.target map key that declares the distinct hook binary.
type targetHookEntry struct {
	Alias      string
	Function   string
	Group      string
	Version    string
	Kind       string
	TargetName string
}

// targetRecEntry represents one TargetReconcilerRegistry assignment in the generated file.
type targetRecEntry struct {
	Alias      string
	Function   string
	Group      string
	Version    string
	Kind       string
	TargetName string
}

// Docs and dashboards
type CRDMeta struct {
	Name        string
	Description string

	Group      string
	Version    string
	Kind       string
	Plural     string
	Namespaced bool
	Namespace  string

	Workers int
	Resync  string

	DependsOn []string

	Reconciler struct {
		Default  bool
		Function string // for custom reconcilers
	}

	Queue struct {
		MaxDepth int
		Shared   bool
	}

	API struct {
		Object string
		List   string
		Alias  string
		Import string
	}
}

type DashboardTemplateData struct {
	Timestamp time.Time
	CRD       CRDMeta
}
