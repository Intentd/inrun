package registry

import (
	"time"

	"github.com/intentd/inrun/pkg/util/term"
)

// PatternKind is the Inrun pattern type, read from the kind: field.
type PatternKind string

// IsBlueprint reports whether this is a Blueprint-based operator pattern.
func (k PatternKind) IsBlueprint() bool { return k == BlueprintKind }

// IsModule reports whether this is a reusable resource primitive.
func (k PatternKind) IsModule() bool { return k == ModuleKind }

// SupportsGates reports whether patterns of this kind can carry E2E and
// Simulate verification gates with optional intent play. Both gates are
// declared in test/ files next to the pattern, which only Blueprints have today.
func (k PatternKind) SupportsGates() bool { return k.IsBlueprint() }

// PatternSpec describes the conventions for one pattern kind.
type PatternSpec struct {
	Kind          PatternKind
	MediaType     string
	PrimaryFile   string
	RequiredFiles []string
	OptionalFiles []string
}

// PatternMeta holds metadata for an pattern — read from its primary YAML on
// push, and reconstructed from OCI annotations on info/list.
type PatternMeta struct {
	Kind           PatternKind
	Name           string
	Version        string
	Description    string
	Author         string
	License        string
	Tags           []string
	RuntimeVersion string             // inrun version (declarative) or go.mod inrun version (typed)
	E2E            *PatternE2E        // populated at push time from running inrun e2e; nil when absent
	Simulate       *PatternSimulate   // populated at push time from running simulate gate; nil when absent
	Intent         *PatternIntent     // populated at push time from running inrun serve play; nil when absent
	Typed          *PatternTyped      // populated at push time from inspecting blueprint CRDs; nil for modules or non-typed blueprints
	Deprecated     *PatternDeprecated // populated from metadata.deprecation in the source YAML; nil when absent
	Replaces       string             // digest of the artifact this push replaced at the same version (push --overwrite); "" when none
}

// PatternTyped carries the typed-operator annotation flags for a blueprint.
type PatternTyped struct {
	HasHooks       bool // one or more CRDs declare customHooks
	HasConstructor bool // one or more CRDs declare customConstructor
}

// PatternDeprecated carries the deprecation metadata declared in the source YAML.
type PatternDeprecated struct {
	MigratedTo   string // full OCI ref of the replacement version
	Message      string // human-readable migration guidance
	TimelineFrom string // YYYY-MM-DD — warn from this date
	TimelineTo   string // YYYY-MM-DD — EOL on this date
}

// PatternE2E holds E2E verification metadata embedded in OCI annotations at push time.
type PatternE2E struct {
	Status     Status // "passed", "skipped"
	Duration   string // e.g. "45s"
	TestedAt   string // RFC3339
	Runner     string // "local" or "github-actions"
	Assertions int    // total number of expectations run; 0 when skipped
	Passed     int    // number of passed expectations
	Failed     int    // number of failed expectations
	Skipped    int    // number of skipped expectations
}

// PatternIntent holds serve play verification metadata embedded in OCI annotations at push time.
type PatternIntent struct {
	Status   Status // "passed" / "failed"
	Target   string // target name played
	TestedAt string // RFC3339
}

// PatternSimulate holds simulate gate metadata embedded in OCI annotations at push time.
// Status values:
//   - "passed"       — simulate.yaml had expect: blocks and all assertions passed
//   - "no-assertion" — simulate.yaml present but no expect: block; ran clean, nothing asserted
//   - "skipped"      — --no-simulate or --force used
type PatternSimulate struct {
	Status     Status // "passed", "no-assertion", "skipped"
	Duration   string // e.g. "4ms"; empty for skipped/no-assertion
	TestedAt   string // RFC3339
	Assertions int    // number of assertions declared in simulate.yaml; 0 when skipped or no-assertion
	Passed     int    // number of passed expectations
	Failed     int    // number of failed expectations
	Skipped    int    // number of skipped expectations
}

// PatternEntry is one row in the pattern index.
type PatternEntry struct {
	Name           string   `json:"name"`
	LatestVersion  string   `json:"latestVersion"`
	Description    string   `json:"description"`
	Tags           []string `json:"tags"`
	Author         string   `json:"author,omitempty"`
	Kind           string   `json:"kind,omitempty"`           // "Blueprint" or "Module"
	E2EStatus      string   `json:"e2eStatus,omitempty"`      // "passed", "skipped", or ""
	SimulateStatus string   `json:"simulateStatus,omitempty"` // "passed", "skipped", "no-assertion", or ""
	Deprecated     bool     `json:"deprecated,omitempty"`
}

// PushOptions carries optional metadata baked into OCI annotations at push time.
type PushOptions struct {
	E2E            *PatternE2E
	Simulate       *PatternSimulate
	Intent         *PatternIntent
	Typed          *PatternTyped
	RuntimeVersion string
	Replaces       string // digest of the artifact being replaced at the same version, recorded in the new one
}

// PatternIndex is the top-level index stored at registry/index:latest.
type PatternIndex struct {
	UpdatedAt string         `json:"updatedAt"`
	Entries   []PatternEntry `json:"entries"`
}

// FileEntry is one file published in an OCI artifact layer.
type FileEntry struct {
	Name   string
	Size   int64
	Digest string // OCI layer digest — used to fetch blob content via ViewFile
}

// PatternInfo holds the metadata returned by Info.
type PatternInfo struct {
	Ref      *Ref
	Digest   string
	Size     int64
	PushedAt time.Time
	Meta     *PatternMeta
	Files    []FileEntry
}

// VersionInfo is a lightweight entry returned by ListVersions.
type VersionInfo struct {
	Tag      string
	PushedAt time.Time
	Meta     *PatternMeta
	Digest   string
}

// Status is the outcome of a verification step (e2e, simulate, intent play).
// It serializes as a stable string in OCI annotations and renders with a
// display symbol in `inrun inspect`.
type Status string

const (
	StatusPassed  Status = "passed"
	StatusSkipped Status = "skipped"
	StatusFailed  Status = "failed"
	// StatusNoAssertion means the run completed but had nothing to check —
	// recorded so `inrun inspect` shows a gate that ran without asserting.
	StatusNoAssertion Status = "no-assertion"
	// StatusNotRun is the zero value: no verification was attempted.
	StatusNotRun Status = ""
)

// String returns the annotation value (stable; do not change).
func (s Status) String() string { return string(s) }

func ToStatus(s string) Status { return Status(s) }

// Symbol returns the display glyph used by `inrun inspect`.
func (s Status) Symbol() string {
	switch s {
	case StatusPassed:
		return term.SuccessMark()
	case StatusSkipped:
		return term.SkippedMark()
	case StatusFailed:
		return term.FailureMark()
	case StatusNoAssertion:
		return term.EmptyMark()
	default:
		return term.NoneMark()
	}
}

// GateStatus is the shape e2eLine needs from any verification gate
// (*PatternE2E, *PatternSimulate). Kept unexported: it exists to share one
// renderer, not as a public contract.
type GateStatus interface {
	GateStatus() Status
	GateDuration() string
	GateCounts() (failed, total int)
}

func (p *PatternE2E) GateStatus() Status     { return p.Status }
func (p *PatternE2E) GateDuration() string   { return p.Duration }
func (p *PatternE2E) GateCounts() (int, int) { return p.Failed, p.Assertions }

func (p *PatternSimulate) GateStatus() Status     { return p.Status }
func (p *PatternSimulate) GateDuration() string   { return p.Duration }
func (p *PatternSimulate) GateCounts() (int, int) { return p.Failed, p.Assertions }

var _ GateStatus = (*PatternE2E)(nil)
var _ GateStatus = (*PatternSimulate)(nil)
