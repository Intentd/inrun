//go:build !runtime && !gateway

package testsuite

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/intentd/inrun/cmd/cli/cmdutil"
	"github.com/intentd/inrun/pkg/blueprint"
	"github.com/intentd/inrun/pkg/merger"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// readSpecCRs reads a spec's CR files, relative to dir, grouped by kind.
func readSpecCRs(dir string, paths []string) (map[string][]*unstructured.Unstructured, error) {
	var buf []byte
	for _, p := range paths {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		data, err := cmdutil.ReadLocal(p)
		if err != nil {
			return nil, fmt.Errorf("reading CR %s: %w", p, err)
		}
		if len(buf) > 0 {
			buf = append(buf, '\n')
		}
		buf = append(buf, data...)
	}
	if len(buf) == 0 {
		return nil, fmt.Errorf("spec.cr, spec.crFiles or spec.intent is required")
	}
	crs, err := cmdutil.ParseMultiDocCRs(buf)
	if err != nil {
		return nil, err
	}
	if len(crs) == 0 {
		return nil, fmt.Errorf("no valid CR documents in CR file(s)")
	}
	return crs, nil
}

// PlayIntent builds a CR from an intent file the way `inrun serve play` does,
// returning the constructed object. The serve package owns the implementation
// and assigns this in init, because that package owns the stage chain.
//
// With Verbose set every stage is printed; without it nothing is, and an error
// is returned for the caller to report. The CR is built the same either way.
//
// Set once during init and read-only thereafter.
var PlayIntent func(PlayRequest) (*unstructured.Unstructured, error)

type PlayRequest struct {
	Blueprint     *blueprint.Blueprint // the blueprint the intent is resolved against
	BlueprintFile string               // path to the blueprint.yaml
	IntentFile    string               // path to the intent YAML/JSON
	Token         string               // serve token to check the op against
	Op            types.ServeOperation // create / update / delete
	Cycles        int                  // reconcile cycles for simulate
	Verbose       bool                 // print each stage
}

// BlueprintCRDPaths returns the CRD files the Blueprint's CRDs point at with
// crdFile, resolved like the Blueprint parser does. --envtest installs them when
// a Simulate spec lists no crd or crdFiles.
func BlueprintCRDPaths(m *merger.Merger) []string {
	dir := m.FirstEntryDir()
	seen := map[string]bool{}
	var paths []string
	for _, crd := range m.Enabled() {
		p := crd.CRDFile
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		if !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}
