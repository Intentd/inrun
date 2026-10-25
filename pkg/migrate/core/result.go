package core

// CRResult is the outcome for one CR.
type CRResult struct {
	Namespace     string
	Name          string
	Fields        []string               // migrated or restored fields
	DriftedFields []string               // fields left alone because they changed after migration
	AllDrifted    bool                   // snapshot present, everything drifted, nothing restored
	Drifted       bool                   // kind level: destination exists and differs
	Pruned        bool                   // kind level: source deleted (or would be, in dry-run)
	AfterSpec     map[string]interface{} // spec after the change; nil if skipped or failed
	Skipped       bool                   // nothing to do
	Excluded      bool                   // failed migrate.selection when/or; also Skipped
	Err           error
}

// Ref returns "namespace/name", or "name" when cluster-scoped.
func (r CRResult) Ref() string {
	return MetaRef(r.Namespace, r.Name)
}

// KindResult is the outcome for all CRs of one kind.
type KindResult struct {
	Kind string
	// To is the destination kind of a kind-level migration; empty otherwise.
	To string
	// Selection is the scope the run used: declared selection merged with
	// the CLI flags.
	Selection Selection
	CRs       []CRResult
}
