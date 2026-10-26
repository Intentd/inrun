//go:build !runtime && !gateway

package cmdutil

import (
	"bytes"
	"encoding/json"

	sigsyaml "sigs.k8s.io/yaml"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ParseMultiDocCRs splits a YAML file on document separators and returns all
// valid CR documents grouped by lowercase kind, in file order. Supports
// single- and multi-doc CR files (multiple CRs separated by ---).
//
// Multiple documents of the SAME kind are all kept (not last-one-wins) —
// see resolveCRInputs for how the extras are used.
func ParseMultiDocCRs(data []byte) map[string][]*unstructured.Unstructured {
	crs := map[string][]*unstructured.Unstructured{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var node yaml.Node
		if err := dec.Decode(&node); err != nil {
			break // EOF or parse error
		}
		// YAML → JSON → Unstructured: go-yaml can produce non-JSON types
		// (!!timestamp, int64, map[interface{}]interface{}) that Unstructured's
		// map[string]interface{} does not accept. YAMLToJSON normalises them to
		// JSON-compatible types before json.Unmarshal populates the object.
		b, err := yaml.Marshal(&node)
		if err != nil {
			continue
		}
		j, err := sigsyaml.YAMLToJSON(b)
		if err != nil {
			continue
		}
		var cr unstructured.Unstructured
		if err := json.Unmarshal(j, &cr.Object); err != nil {
			continue
		}
		if kind := cr.GetKind(); kind != "" {
			key := CRKey(kind)
			crs[key] = append(crs[key], &cr)
		}
	}
	return crs
}

// CRSimInputs is one target CRD's resolved simulation inputs, derived from a
// multi-doc CR file by resolveCRInputs.
type CRSimInputs struct {
	CR       *unstructured.Unstructured            // the CR under test — reconciled
	Peers    map[string]*unstructured.Unstructured // other kinds, first doc each — for cross:
	Existing []*unstructured.Unstructured          // same kind, docs after the first — NOT reconciled
}

// ResolveCRInputs picks the CR to reconcile for crdEntry's kind out of a
// multi-doc CR file, plus its simulation context.
//
// The FIRST document of the target kind is the CR under test — unchanged
// behavior from a single-CR file. Any FURTHER documents of that same kind
// are pre-existing instances: seeded into the fake dynamic client so
// reconcile-time checks that list other instances (unique: true) can
// see them, but never reconciled themselves — same convention already used
// for cross: peers, extended from "one doc per other kind" to "one or more
// docs of the CRD's own kind". Documents of OTHER kinds become cross: peers,
// one per kind (first document wins), exactly as before.
func ResolveCRInputs(crs map[string][]*unstructured.Unstructured, kind string) (CRSimInputs, bool) {
	kindKey := CRKey(kind)
	docs, ok := crs[kindKey]
	if !ok || len(docs) == 0 {
		return CRSimInputs{}, false
	}
	out := CRSimInputs{CR: docs[0], Existing: docs[1:]}
	if len(crs) > 1 {
		out.Peers = make(map[string]*unstructured.Unstructured, len(crs)-1)
		for k, v := range crs {
			if k != kindKey && len(v) > 0 {
				out.Peers[k] = v[0]
			}
		}
	}
	return out, true
}
