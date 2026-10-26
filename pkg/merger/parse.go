// pkg/merger/parse.go
package merger

import (
	"fmt"
	"strings"

	"github.com/intentd/inrun/pkg/config"
	"github.com/intentd/inrun/pkg/types"
)

// parseBlueprintDoc parses a single YAML document and returns a BlueprintFile
// if it is a valid Blueprint or Stack. Returns (nil, nil) for any other
// document — not an error, just not a document we care about.
//
// Hard errors only when the document looks like a Blueprint/Stack
// (passes the fast string check) but fails to parse or has an unsupported
// apiVersion. Silently skipping malformed documents would hide user mistakes.
func parseBlueprintDoc(doc []byte, source string) (*types.BlueprintFile, error) {
	// Fast path — skip documents that contain neither Blueprint nor Stack kind.
	// This avoids full YAML parsing for every non-relevant template in a Helm manifest.
	if !containsValidKind(doc) {
		return nil, nil
	}

	// Detect list-format CRDs before full parsing — gives a clear error instead of
	// a raw YAML unmarshal failure. spec.crds must be a map, not a sequence:
	//   spec.crds:             ← correct (map)
	//     myresource: {}
	//   spec.crds:             ← wrong (list)
	//     - name: myresource
	if looksLikeCRDList(doc) {
		return nil, fmt.Errorf(
			"%q: spec.crds must be a map (name: {}) not a list (- name:).\n"+
				"  See: https://inrun.dev/reference/blueprint#spec-crds",
			source,
		)
	}

	var blueprint types.BlueprintFile
	if err := strictUnmarshal(doc, &blueprint); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", source, err)
	}

	// Kind must be Blueprint or Stack — anything else is silently skipped.
	// This handles YAML files that happen to contain the word "Blueprint" in a comment.
	if !config.IsValidPatternKind(blueprint.Kind) {
		return nil, nil
	}

	// apiVersion must match a supported version — hard error with guidance.
	if !config.IsValidApiVersion(blueprint.APIVersion) {
		return nil, fmt.Errorf(
			"%q: unsupported apiVersion %q\n"+
				"  Supported: %v\n"+
				"  This usually means the pattern was built for a different version of Inrun.\n"+
				"  Check the upstream pattern's blueprint.yaml or update Inrun.",
			source, blueprint.APIVersion, config.ApiVersions(),
		)
	}

	// Name is required irrespective of kind
	if blueprint.Metadata.Name == "" {
		return nil, fmt.Errorf("%q: missing metadata.name", source)
	}

	return &blueprint, nil
}

// looksLikeCRDList detects the common mistake of writing spec.crds as a YAML
// list instead of a map. Checks for "- name:" immediately after "crds:" with
// optional whitespace — good enough to catch the pattern without full parsing.
func looksLikeCRDList(doc []byte) bool {
	s := string(doc)
	idx := strings.Index(s, "crds:")
	if idx < 0 {
		return false
	}
	// Look at the content after "crds:" — if the next non-blank line starts
	// with "- " it is a list entry.
	after := strings.TrimLeft(s[idx+5:], " \t\r\n")
	return strings.HasPrefix(after, "- ")
}

// containsValidKind is a fast string check before committing to a full YAML parse.
// Returns true if the document contains either "kind: Blueprint" or "kind: Stack".
func containsValidKind(doc []byte) bool {
	s := string(doc)
	return strings.Contains(s, fmt.Sprintf("kind: %s", config.BlueprintKind())) ||
		strings.Contains(s, fmt.Sprintf("kind: %s", config.StackKind()))
}

// sniffDocumentKind does a fast string scan for known Inrun kinds that are
// NOT valid merger inputs (Module, E2E). Used to produce a clear error instead
// of silently treating the file as an empty Blueprint.
// Returns the detected kind string, or "" if none recognized.
func sniffDocumentKind(doc []byte) string {
	s := string(doc)
	for _, kind := range []string{config.ModuleKind(), config.E2EKind()} {
		if strings.Contains(s, fmt.Sprintf("kind: %s", kind)) {
			return kind
		}
	}
	return ""
}

// Export
var ParseBlueprintDoc = parseBlueprintDoc
