// pkg/config/inrun.go
package config

import "time"

// ── Runtime ───────────────────────────────────────────────────────────

// Runtime returns the instance identifier
// Used by pkg/inrun and other packages to determine what is running.
func Runtime() Instance {
	return InstanceRuntime
}

// Gateway returns the instance identifier for the gateway service,
// Used by pkg/inrun and other packages to determine what is running.
func Gateway() Instance {
	return InstanceGateway
}

// String returns the string representation of the Instance, suitable for
// logging, printing, and serialization.
func (i Instance) String() string {
	return string(i)
}

// ── Inrun Config ─────────────────────────────────────────────────────────

// SetName sets the configured name for this Inrun instance.
func (c *inrunConfig) SetName(v string) {
	c.name = v
}

// Name returns the configured name for this Inrun instance.
func (c *inrunConfig) Name() string {
	return c.name
}

// Instance returns the configured Instance (runtime or gateway) for this config.
func (c *inrunConfig) Instance() Instance {
	return c.instance
}

// ShortName returns the short name used for display or compact identifiers.
func (c *inrunConfig) ShortName() string {
	return c.shortName
}

// Environment returns the environment label (dev, staging, production, etc.).
func (c *inrunConfig) Environment() string {
	return c.environment
}

// LogLevel returns the configured logging level for the process.
func (c *inrunConfig) LogLevel() string {
	return c.logLevel
}

// ── Leader Election ───────────────────────────────────────────────────────────

// Namespace returns the election namespace.
func (e *leaderElection) Namespace() string {
	return e.namespace
}

// SetNamespace sets the election namespace.
func (e *leaderElection) SetNamespace(v string) {
	e.namespace = v
}

// LeaseDuration returns the configured lease duration for leader election.
func (e *leaderElection) LeaseDuration() time.Duration {
	return e.leaseDuration
}

// SetLeaseDuration sets the lease duration for leader election.
func (e *leaderElection) SetLeaseDuration(v time.Duration) {
	e.leaseDuration = v
}

// RenewDeadline returns the renew deadline used in leader election.
func (e *leaderElection) RenewDeadline() time.Duration {
	return e.renewDeadline
}

// SetRenewDeadline sets the renew deadline used in leader election.
func (e *leaderElection) SetRenewDeadline(v time.Duration) {
	e.renewDeadline = v
}

// RetryPeriod returns the retry period used in leader election.
func (e *leaderElection) RetryPeriod() time.Duration {
	return e.retryPeriod
}

// SetRetryPeriod sets the retry period used in leader election.
func (e *leaderElection) SetRetryPeriod(v time.Duration) {
	e.retryPeriod = v
}

// BlueprintKind returns the kind string for a Blueprint document.
// Blueprints declare CRDs in spec.crds. No sources block.
func BlueprintKind() string {
	return kindBlueprint
}

// StackKind returns the kind string for a Stack document.
// Stacks compose Blueprints from sources (files, helm).
func StackKind() string {
	return kindStack
}

// ModuleKind returns the kind string for a Module document.
func ModuleKind() string {
	return kindModule
}

// E2EKind returns the kind string for an E2E document.
func E2EKind() string {
	return kindE2E
}

// SimulateKind returns the kind string for a Simulate document.
func SimulateKind() string {
	return kindSimulate
}

// KonduktorKind returns the kind string for a Konduktor document.
func KonduktorKind() string {
	return kindLeader
}

// IsBlueprintKind returns true if the given kind is a Blueprint.
func IsBlueprintKind(kind string) bool {
	return kind == kindBlueprint
}

// IsKonduktorKind returns true if the given kind is a Konduktor.
func IsKonduktorKind(kind string) bool {
	return kind == kindLeader
}

// IsModuleKind returns true if the given kind is a Module.
func IsModuleKind(kind string) bool {
	return kind == kindModule
}

// IsStackKind returns true if the given kind is a Stack.
func IsStackKind(kind string) bool {
	return kind == kindStack
}

// IsE2EKind returns true if the given kind is an E2E.
func IsE2EKind(kind string) bool {
	return kind == kindE2E
}

// IsSimulateKind returns true if the given kind is a Simulate.
func IsSimulateKind(kind string) bool {
	return kind == kindSimulate
}

// IsValidPatternKind reports whether the given kind is one of the supported
// Inrun pattern kinds.
func IsValidPatternKind(kind string) bool {
	return kind == kindBlueprint ||
		kind == kindStack ||
		kind == kindModule ||
		kind == kindE2E ||
		kind == kindSimulate
}

// ValidKindsString returns a comma‑separated list of all supported document kinds.
// Useful for error messages and CLI diagnostics.
func ValidKindsString() string {
	return "Blueprint, Stack, Module, E2E, Simulate"
}

// IsValidApiVersion returns true if the given apiVersion is a supported version.
func IsValidApiVersion(apiVersion string) bool {
	for _, v := range apiVersions {
		if v == apiVersion {
			return true
		}
	}
	return false
}

// ApiVersions returns the list of supported apiVersions.
func ApiVersions() []string {
	return apiVersions
}
