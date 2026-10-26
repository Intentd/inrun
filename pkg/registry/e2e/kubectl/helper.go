package kubectl

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/intentd/inrun/pkg/util/fileio"
	"github.com/intentd/inrun/pkg/util/structured"
	"github.com/intentd/inrun/pkg/util/term"
	"github.com/intentd/kel/core"
	"gopkg.in/yaml.v3"

	"github.com/intentd/inrun/pkg/types"
)

var (
	readLocal         = fileio.ReadLocal
	toAbs             = fileio.ToAbs
	startSpinner      = term.StartSpinner
	parseTimeDuration = core.ParseTimeDuration
	applyExtract      = types.ApplyExtract
	requireErr        = structured.RequireErr
)

// assertionListMsg is appended to every "at least one assertion required"
// validation error; the names come from types.AssertionNames, plus the
// JSON extraction/assertion blocks that count as assertions on their own.
var assertionListMsg = "at least one assertion required (" + strings.Join(types.AssertionNames(), ", ") + ", text:, json:)"

// defaultOr returns ns, or "default" when empty. Every kubectl verb
// that takes -n uses it, so the default lives in one place.
func defaultOr(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

// hasAssertion reports whether t, j, or a declares at least one assertion.
// Pass a nil j (or zero-length) when the entry type has no inline JSON field.
func hasAssertion(t types.TextAssertion, a *types.Assertions, j ...types.JSONAssertions) bool {
	if !(types.Assertions{TextAssertion: t}).Empty() {
		return true
	}
	for _, jj := range j {
		if len(jj) > 0 {
			return true
		}
	}
	return a != nil && !a.Empty()
}

// assertInlineJSON checks inline j assertions against out, prefixing failures
// with ctx. A nil/empty j is a no-op.
func assertInlineJSON(out string, j types.JSONAssertions, ctx string) error {
	if len(j) == 0 {
		return nil
	}
	if err := (types.Assertions{JSON: j}).Check(out); err != nil {
		return fmt.Errorf("%s: %w\noutput: %s", ctx, err, out)
	}
	return nil
}

// assertOutput checks out against a, prefixing a failure with ctx and the
// output.
func assertOutput(out string, t types.TextAssertion, a *types.Assertions, ctx string) error {
	if inline := (types.Assertions{TextAssertion: t}); !inline.Empty() {
		if err := inline.Check(out); err != nil {
			return fmt.Errorf("%s: %w\noutput: %s", ctx, err, out)
		}
	}
	if a == nil {
		return nil
	}
	if err := a.Check(out); err != nil {
		return fmt.Errorf("%s: %w\noutput: %s", ctx, err, out)
	}
	return nil
}

// checkFile reports a local file a step names that is not there, resolved
// from baseDir, the directory the step runs from. URLs and templated paths
// are left to the run.
func checkFile(loc, field, ref, baseDir string) []error {
	if baseDir == "" {
		return nil
	}
	path, ok := fileio.LocalRef(baseDir, ref)
	if !ok {
		return nil
	}
	if err := fileio.CheckLocal(path); err != nil {
		return []error{fmt.Errorf("%s: %s %q: %w", loc, field, ref, err)}
	}
	return nil
}

// checkSingleObject reports a manifest that names other than one object.
// A missing file is checkFile's to report.
func checkSingleObject(loc, ref, baseDir string) []error {
	if baseDir == "" {
		return nil
	}
	path, ok := fileio.LocalRef(baseDir, ref)
	if !ok {
		return nil
	}
	data, err := readLocal(path)
	if err != nil {
		return nil
	}
	n := 0
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return []error{fmt.Errorf("%s: file %q: %w", loc, ref, err)}
		}
		if len(doc) > 0 {
			n++
		}
	}
	if n != 1 {
		return []error{fmt.Errorf("%s: file %q is both target and patch, so it must hold one object; it holds %d (use patch or patchFile for the patch)", loc, ref, n)}
	}
	return nil
}
