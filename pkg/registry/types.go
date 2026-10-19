package registry

import "time"

// PatternKind is the Inrun pattern type, read from the kind: field.
type PatternKind string

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
	Status     string // "passed", "skipped"
	Duration   string // e.g. "45s"
	TestedAt   string // RFC3339
	Runner     string // "local" or "github-actions"
	Assertions int    // total number of expectations run; 0 when skipped
}

// PatternIntent holds serve play verification metadata embedded in OCI annotations at push time.
type PatternIntent struct {
	Status   string // "passed" / "failed"
	Target   string // target name played
	TestedAt string // RFC3339
}

// PatternSimulate holds simulate gate metadata embedded in OCI annotations at push time.
// Status values:
//   - "passed"       — simulate.yaml had expect: blocks and all assertions passed
//   - "no-assertion" — simulate.yaml present but no expect: block; ran clean, nothing asserted
//   - "skipped"      — --no-simulate or --force used
type PatternSimulate struct {
	Status     string // "passed", "no-assertion", "skipped"
	Duration   string // e.g. "4ms"; empty for skipped/no-assertion
	TestedAt   string // RFC3339
	Assertions int    // number of assertions declared in simulate.yaml; 0 when skipped or no-assertion
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
