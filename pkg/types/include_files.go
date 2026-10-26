package types

// Include files: the shape of each file an include: field points at. Every
// loader decodes its file strictly into the type here, and hack/generate-schema
// writes an editor schema for each, named in IncludeSchemas.

// StatusInclude is the file status.include names: fields added before the
// inline ones.
type StatusInclude struct {
	Fields StatusFieldSpecs `yaml:"fields"`
}

// ValidationInclude is the file admission.validation.include names: rules
// added before the inline ones.
type ValidationInclude struct {
	Rules ValidationRules `yaml:"rules"`
}

// MutationInclude is the file admission.mutation.include names: rules added
// before the inline ones.
type MutationInclude struct {
	Rules MutationRules `yaml:"rules"`
}

// ConversionInclude is the file conversion.include names: paths added before
// the inline ones.
type ConversionInclude struct {
	Paths ConversionPaths `yaml:"paths"`
}

// FunctionsInclude is the file functions.include names: definitions added
// before the inline ones.
type FunctionsInclude struct {
	Definitions UserDefinedFunctions `yaml:"definitions"`
}

// PresetsInclude is the file presets.include names: each list added before the
// inline entries.
type PresetsInclude struct {
	Presets PresetRegistry `yaml:"presets"`
}

// ObserveInclude is the file observe.include names: watches added before the
// inline ones, and events the inline ones override by name.
type ObserveInclude struct {
	Watch  WatchEntries          `yaml:"watch"`
	Events map[string]EventEntry `yaml:"events"`
}

// ReconcilerInclude is the file reconciler.include names: a reconciler block
// the inline fields override.
type ReconcilerInclude struct {
	Reconciler ReconcilerConfig `yaml:"reconciler"`
}

// ReconcileInclude is the file reconcile.include names: a reconcile block the
// inline fields override.
type ReconcileInclude struct {
	Reconcile ReconcileConfig `yaml:"reconcile"`
}

// ServeInclude is the file serve.include names: fields, labels and annotations
// the inline ones override by name.
type ServeInclude struct {
	Fields      map[string]ServeFieldConfig `yaml:"fields"`
	Labels      map[string]ServeFieldConfig `yaml:"labels"`
	Annotations map[string]ServeFieldConfig `yaml:"annotations"`
}

// GatewayAuthInclude is the file gateway.api.auth.include names: tokens the
// inline ones override by name.
type GatewayAuthInclude struct {
	Tokens APITokens `yaml:"tokens"`
}

// GatewayClustersInclude is the file gateway.clusters.include names: clusters
// the inline ones override by name.
type GatewayClustersInclude struct {
	Clusters map[string]GatewayClusterConfig `yaml:"clusters"`
}

// IsGatewayClustersIncludeDoc returns true when the file is a valid Gateway cluster expect include.
func IsGatewayClustersIncludeDoc(path string) (bool, error) {
	data, err := readLocal(path)
	if err != nil {
		return false, err
	}

	var include ExternalCallsInclude
	if err := strictUnmarshal(data, &include); err != nil {
		return false, err
	}

	return true, nil
}

// ExternalCallsInclude is the file an include: entry in a Blueprint's external
// calls names: the calls that replace the entry.
type ExternalCallsInclude struct {
	Calls ExternalCallSpecs `yaml:"calls"`
}

// IsExternalCallsIncludeDoc returns true when the file is a valid E2E expect include.
func IsExternalCallsIncludeDoc(path string) (bool, error) {
	data, err := readLocal(path)
	if err != nil {
		return false, err
	}

	var include ExternalCallsInclude
	if err := strictUnmarshal(data, &include); err != nil {
		return false, err
	}

	return true, nil
}

// SimulateOpsRuleInclude is the document referenced by an include: entry
// in a Simulate ops or absent list.
type SimulateOpsRuleInclude struct {
	Ops    SimulateOpRules `yaml:"ops"`
	Absent SimulateOpRules `yaml:"absent"`
}

// IsSimulateOpsRuleIncludeDoc returns true when the file is a valid
// Simulate ops/absent include document. Either or both keys may be present.
func IsSimulateOpsRuleIncludeDoc(path string) (bool, error) {
	data, err := readLocal(path)
	if err != nil {
		return false, err
	}

	var include SimulateOpsRuleInclude
	if err := strictUnmarshal(data, &include); err != nil {
		return false, err
	}

	return true, nil
}

// E2EExpectInclude is the file an include: entry in an E2E suite's expect list
// names: the expectations that replace the entry, run from the file's
// directory.
type E2EExpectInclude struct {
	Expect E2EExpectations `yaml:"expect"`
}

// IsE2EExpectIncludeDoc returns true when the file is a valid E2E expect include.
func IsE2EExpectIncludeDoc(path string) (bool, error) {
	data, err := readLocal(path)
	if err != nil {
		return false, err
	}

	var include E2EExpectInclude
	if err := strictUnmarshal(data, &include); err != nil {
		return false, err
	}

	return true, nil
}

// E2EExternalInclude is the file an include: entry in an expectation's
// external checks names: the checks that replace the entry.
type E2EExternalInclude struct {
	Calls E2EExternalChecks `yaml:"calls"`
}

// IsE2EExternalIncludeDoc returns true when the file is a valid E2E external check include.
func IsE2EExternalIncludeDoc(path string) (bool, error) {
	data, err := readLocal(path)
	if err != nil {
		return false, err
	}

	var include E2EExternalInclude
	if err := strictUnmarshal(data, &include); err != nil {
		return false, err
	}

	return true, nil
}

// IncludeSchemas maps each include file's schema, under SchemaBaseURL, to the
// type it is generated from. A serve target's include file holds a target
// config, so its schema comes from ServeTargetConfig.
var IncludeSchemas = map[string]any{
	"include/status.json":            StatusInclude{},
	"include/validation.json":        ValidationInclude{},
	"include/mutation.json":          MutationInclude{},
	"include/conversion.json":        ConversionInclude{},
	"include/functions.json":         FunctionsInclude{},
	"include/presets.json":           PresetsInclude{},
	"include/observe.json":           ObserveInclude{},
	"include/reconciler.json":        ReconcilerInclude{},
	"include/reconcile.json":         ReconcileInclude{},
	"include/serve.json":             ServeInclude{},
	"include/serve-target.json":      ServeTargetConfig{},
	"include/gateway-auth.json":      GatewayAuthInclude{},
	"include/gateway-clusters.json":  GatewayClustersInclude{},
	"include/external-calls.json":    ExternalCallsInclude{},
	"include/simulate-ops-rule.json": SimulateOpsRuleInclude{},
	"include/e2e-expect.json":        E2EExpectInclude{},
	"include/e2e-external.json":      E2EExternalInclude{},
}
