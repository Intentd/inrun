package simulate

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/intentd/inrun/pkg/registry"
	"github.com/intentd/inrun/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Comparison is the outcome of simulating one CR in memory and on envtest
// from the same input: whether the two wrote the same things and left the
// cluster holding the same objects.
type Comparison struct {
	// Name is the CRD simulated.
	Name string
	// Memory and Envtest are the two runs; nil when a run failed to execute.
	Memory, Envtest *Result
	// MemoryErr and EnvtestErr are the runs' execution errors.
	MemoryErr, EnvtestErr error
	// Writes compares what each run wrote.
	Writes WriteComparison
	// Objects compares the CR and its children after the last cycle, one
	// entry per object either run holds.
	Objects []ObjectComparison
}

// WriteComparison compares the writes two runs recorded, by verb, resource,
// namespace and name, across all cycles: a write's cycle is diagnostic, not
// a difference.
type WriteComparison struct {
	// Matched is how many distinct writes both runs made equally often.
	Matched int `json:"matched"`
	// Differences are the writes one run made more often than the other,
	// including never.
	Differences []WriteDiff `json:"differences,omitempty"`
}

// WriteDiff is one write two runs made a different number of times.
type WriteDiff struct {
	Verb string `json:"verb"`
	// Group is the API group of the resource written, or of the object a
	// write to part of one (status, labels, ...) patched.
	Group     string `json:"group,omitempty"`
	Resource  string `json:"resource"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	// Memory and Envtest are how often each run made the write.
	Memory  int `json:"memory"`
	Envtest int `json:"envtest"`
	// MemoryCycles and EnvtestCycles are the cycles each made it in.
	MemoryCycles  []int `json:"memoryCycles,omitempty"`
	EnvtestCycles []int `json:"envtestCycles,omitempty"`

	// qualify — another differing write has the same resource, namespace
	// and name in another group, so Ref names the group.
	qualify bool
}

// Ref names the write's object as resource/name, the way ops are printed,
// with the API group when another write would read the same.
func (d WriteDiff) Ref() string {
	return qualifiedRef(d.Namespace, d.Resource, d.Group, d.Name, d.qualify)
}

// qualifiedRef is namespace/kind/name, with kind.group (as kubectl prints a
// resource) when qualify.
func qualifiedRef(namespace, kind, group, name string, qualify bool) string {
	if qualify && group != "" {
		kind += "." + group
	}
	ref := kind + "/" + name
	if namespace != "" {
		ref = namespace + "/" + ref
	}
	return ref
}

// ObjectComparison compares one object as the two runs left it.
type ObjectComparison struct {
	// Group, Kind, Namespace and Name identify the object.
	Group, Kind, Namespace, Name string
	// InMemory and InEnvtest say which runs hold it.
	InMemory, InEnvtest bool
	// Differences are fields whose values differ, that only the in-memory
	// run holds (envtest pruned or never stored them), or that only envtest
	// holds because a server-side apply sent them.
	Differences []FieldDiff
	// Defaulted are fields only envtest holds that no server-side apply owns:
	// set by the API server's defaulting, which the in-memory cluster does
	// not run. They are reported, not counted as differences.
	Defaulted []FieldDiff
	// Varies are fields whose values differ because the Blueprint fills
	// them from a random function (Result.RandomSecretKeys): different in
	// every run by design. They are reported, not counted as differences.
	Varies []FieldDiff

	// qualify — another compared object has the same kind, namespace and
	// name in another group, so Ref names the group.
	qualify bool
}

// FieldDiffKind says how a field differs between the two runs.
type FieldDiffKind string

const (
	// FieldChanged — both runs hold the field, with different values.
	FieldChanged FieldDiffKind = "changed"
	// FieldPruned — only the in-memory run holds the field.
	FieldPruned FieldDiffKind = "pruned"
	// FieldMissing — only envtest holds the field, and a server-side apply
	// sent it: a write the in-memory run did not make.
	FieldMissing FieldDiffKind = "missing"
	// FieldDefaulted — only envtest holds the field, and no field manager
	// owns it: the API server's defaulting set it.
	FieldDefaulted FieldDiffKind = "defaulted"
	// FieldVaries — both runs hold the field with different values, and the
	// Blueprint fills it from a random function: expected to differ.
	FieldVaries FieldDiffKind = "varies"
)

// FieldDiff is one field that differs; Memory or Envtest is nil when that
// run does not hold it.
type FieldDiff struct {
	Path    string        `json:"path"`
	Kind    FieldDiffKind `json:"kind"`
	Memory  any           `json:"memory,omitempty"`
	Envtest any           `json:"envtest,omitempty"`
}

// Matches reports whether both runs hold the object with no differences.
func (o ObjectComparison) Matches() bool {
	return o.InMemory && o.InEnvtest && len(o.Differences) == 0
}

// Ref names the object as resource/name, the way ops are printed.
func (o ObjectComparison) Ref() string {
	return qualifiedRef(o.Namespace, strings.ToLower(o.Kind), o.Group, o.Name, o.qualify)
}

// Complete reports whether both runs executed, so their results compare.
func (c *Comparison) Complete() bool {
	return c != nil && c.Memory != nil && c.Envtest != nil && c.MemoryErr == nil && c.EnvtestErr == nil
}

// Differences counts what differs: each differing write and each object
// held by one run only or with differing fields. An incomplete comparison
// has none to count.
func (c *Comparison) Differences() int {
	if !c.Complete() {
		return 0
	}
	n := len(c.Writes.Differences)
	for _, o := range c.Objects {
		if !o.Matches() {
			n++
		}
	}
	return n
}

// Equivalent reports whether both runs executed and differ in nothing.
func (c *Comparison) Equivalent() bool {
	return c.Complete() && c.Differences() == 0
}

// Err returns why the comparison fails, or nil: a run that failed to
// execute or whose assertions failed (Evaluate them first), an incomplete
// comparison, or differences between the runs.
func (c *Comparison) Err() error {
	var failures []string
	for _, run := range c.runs() {
		switch {
		case run.err != nil:
			failures = append(failures, run.label+" simulation failed: "+run.err.Error())
		case run.result != nil && run.result.Assertions.Status == registry.StatusFailed:
			failures = append(failures, run.label+" assertions failed")
		}
	}
	diff := c.Differences()
	diffTxt := "difference"
	if diff > 1 {
		diffTxt = "differences"
	}
	switch {
	case !c.Complete():
		failures = append(failures, "comparison incomplete")
	case diff > 0:
		failures = append(failures, fmt.Sprintf("%d %s between in-memory and envtest", diff, diffTxt))
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.New(strings.Join(failures, "; "))
}

// Result folds the comparison into one result, as a single run reports:
// both runs' reconcile time, both runs' assertion cases (labelled with the
// run), and the comparison itself. Its status is failed when Err is set.
func (c *Comparison) Result() *Result {
	out := &Result{Name: c.Name, Comparisons: []*Comparison{c}}
	for _, run := range c.runs() {
		if run.result == nil {
			continue
		}
		a := run.result.Assertions
		out.Elapsed += run.result.Elapsed
		out.Assertions.Total += a.Total
		out.Assertions.Passed += a.Passed
		out.Assertions.Failed += a.Failed
		out.Assertions.Skipped += a.Skipped
		for _, cs := range a.Cases {
			cs.Field = run.label + ": " + cs.Field
			out.Assertions.Cases = append(out.Assertions.Cases, cs)
		}
	}
	out.Assertions.Status = out.SimulateStatusFor()
	if c.Err() != nil {
		out.Assertions.Status = registry.StatusFailed
	}
	return out
}

type comparedRun struct {
	label  string
	result *Result
	err    error
}

func (c *Comparison) runs() []comparedRun {
	return []comparedRun{{"in-memory", c.Memory, c.MemoryErr}, {"envtest", c.Envtest, c.EnvtestErr}}
}

// Compare compares two runs of the same input, memory in memory and envtest
// on envtest. Either may be nil, when its run failed; the comparison is then
// incomplete, and Writes and Objects stay empty.
func Compare(name string, memory, envtest *Result, memoryErr, envtestErr error) *Comparison {
	c := &Comparison{Name: name, Memory: memory, Envtest: envtest, MemoryErr: memoryErr, EnvtestErr: envtestErr}
	if !c.Complete() {
		return c
	}
	c.Writes = compareWrites(memory.AllOps, envtest.AllOps)
	c.Objects = compareObjects(memory.Final, envtest.Final, randomSecretPaths(memory, envtest))
	return c
}

// ── Writes ──────────────────────────────────────────────────────────────────

// writeKey identifies a write. The group is part of it: a CRD may share a
// built-in kind's name (a CronJob CR beside batch CronJobs).
type writeKey struct{ verb, group, resource, namespace, name string }

func compareWrites(memory, envtest Ops) WriteComparison {
	mem, env := countWrites(memory), countWrites(envtest)
	keys := map[writeKey]bool{}
	for k := range mem {
		keys[k] = true
	}
	for k := range env {
		keys[k] = true
	}

	var out WriteComparison
	for k := range keys {
		m, e := mem[k], env[k]
		if len(m) == len(e) {
			out.Matched++
			continue
		}
		out.Differences = append(out.Differences, WriteDiff{
			Verb: k.verb, Group: k.group, Resource: k.resource, Namespace: k.namespace, Name: k.name,
			Memory: len(m), Envtest: len(e), MemoryCycles: m, EnvtestCycles: e,
		})
	}
	sort.Slice(out.Differences, func(i, j int) bool {
		a, b := out.Differences[i], out.Differences[j]
		return fmt.Sprint(a.Resource, a.Namespace, a.Name, a.Group, a.Verb) < fmt.Sprint(b.Resource, b.Namespace, b.Name, b.Group, b.Verb)
	})
	groups := map[string]map[string]bool{}
	for _, d := range out.Differences {
		k := d.Namespace + "/" + d.Resource + "/" + d.Name
		if groups[k] == nil {
			groups[k] = map[string]bool{}
		}
		groups[k][d.Group] = true
	}
	for i, d := range out.Differences {
		out.Differences[i].qualify = len(groups[d.Namespace+"/"+d.Resource+"/"+d.Name]) > 1
	}
	return out
}

// countWrites returns, for each distinct write, the cycle of each time it
// was made. Reads are left out: the envtest recorder sees only writes.
func countWrites(ops Ops) map[writeKey][]int {
	out := map[writeKey][]int{}
	for _, op := range ops {
		if !types.IsWriteVerb(op.Verb) {
			continue
		}
		k := writeKey{op.Verb, op.Group, op.Resource, op.Namespace, op.Name}
		out[k] = append(out[k], op.Cycle)
	}
	return out
}

// ── Objects ─────────────────────────────────────────────────────────────────

type objectKey struct{ group, kind, namespace, name string }

func keyOf(u *unstructured.Unstructured) objectKey {
	gvk := u.GroupVersionKind()
	return objectKey{gvk.Group, gvk.Kind, u.GetNamespace(), u.GetName()}
}

// randomSecretPaths returns the field paths, in a Secret, of the data keys
// either run declares random.
func randomSecretPaths(runs ...*Result) map[string]bool {
	out := map[string]bool{}
	for _, r := range runs {
		for _, key := range r.RandomSecretKeys {
			out[joinPath("data", key)] = true
		}
	}
	return out
}

func compareObjects(memory, envtest []*unstructured.Unstructured, randomSecret map[string]bool) []ObjectComparison {
	memUIDs, envUIDs := uidNames(memory), uidNames(envtest)
	env := map[objectKey]*unstructured.Unstructured{}
	for _, u := range envtest {
		env[keyOf(u)] = u
	}

	var out []ObjectComparison
	done := map[objectKey]bool{}
	for _, m := range memory {
		k := keyOf(m)
		done[k] = true
		oc := ObjectComparison{Group: k.group, Kind: k.kind, Namespace: k.namespace, Name: k.name, InMemory: true}
		if e, ok := env[k]; ok {
			oc.InEnvtest = true
			for _, d := range diffValues("", normalise(m, memUIDs), normalise(e, envUIDs), managedOwner(e)) {
				switch {
				case d.Kind == FieldDefaulted:
					oc.Defaulted = append(oc.Defaulted, d)
				case d.Kind == FieldChanged && k.group == "" && k.kind == "Secret" && randomSecret[d.Path]:
					d.Kind = FieldVaries
					oc.Varies = append(oc.Varies, d)
				default:
					oc.Differences = append(oc.Differences, d)
				}
			}
		}
		out = append(out, oc)
	}
	for _, e := range envtest {
		if k := keyOf(e); !done[k] {
			out = append(out, ObjectComparison{Group: k.group, Kind: k.kind, Namespace: k.namespace, Name: k.name, InEnvtest: true})
		}
	}
	groups := map[string]map[string]bool{}
	for _, o := range out {
		k := o.Namespace + "/" + o.Kind + "/" + o.Name
		if groups[k] == nil {
			groups[k] = map[string]bool{}
		}
		groups[k][o.Group] = true
	}
	for i, o := range out {
		out[i].qualify = len(groups[o.Namespace+"/"+o.Kind+"/"+o.Name]) > 1
	}
	return out
}

// envFields are the metadata fields the API server assigns per cluster.
// They identify a stored copy, not what the operator asked for, so they
// differ between any two clusters and are left out of the comparison.
var envFields = []string{"resourceVersion", "uid", "creationTimestamp", "managedFields", "generation", "selfLink"}

// uidNames maps each object's uid to a name for it that holds in either
// cluster: a uid differs per cluster, the object it names does not.
func uidNames(objs []*unstructured.Unstructured) map[string]string {
	out := map[string]string{}
	for _, u := range objs {
		if uid := string(u.GetUID()); uid != "" {
			out[uid] = "<uid of " + u.GetKind() + "/" + u.GetName() + ">"
		}
	}
	return out
}

// Beyond normalise, diffValues treats two times as equal (each run's clock,
// not its outcome) and compares JSON held in strings field by field.

// normalise returns a JSON-shaped copy of u without the cluster-assigned
// metadata, and with each uid of an object in its run (an owner reference,
// an owner-uid label) replaced by a name for that object. Values pass
// through JSON, so a typed object's int32 and an unstructured int64 compare
// equal. Nulls and empty maps and lists are dropped: Kubernetes reads each
// as absent.
func normalise(u *unstructured.Unstructured, uids map[string]string) any {
	cp := u.DeepCopy()
	for _, f := range envFields {
		unstructured.RemoveNestedField(cp.Object, "metadata", f)
	}
	raw, err := json.Marshal(cp.Object)
	if err != nil {
		return cp.Object
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return cp.Object
	}
	return dropEmpty(replaceUIDs(v, uids))
}

// replaceUIDs replaces each string in v that is a key of uids.
func replaceUIDs(v any, uids map[string]string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			t[k] = replaceUIDs(child, uids)
		}
	case []any:
		for i, child := range t {
			t[i] = replaceUIDs(child, uids)
		}
	case string:
		if name, ok := uids[t]; ok {
			return name
		}
	}
	return v
}

// dropEmpty removes nulls and empty maps and lists, recursively.
func dropEmpty(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if c := dropEmpty(child); c == nil {
				delete(t, k)
			} else {
				t[k] = c
			}
		}
		if len(t) == 0 {
			return nil
		}
		return t
	case []any:
		for i, child := range t {
			t[i] = dropEmpty(child)
		}
		if len(t) == 0 {
			return nil
		}
		return t
	}
	return v
}

// owner is a position in the fields an envtest object's server-side
// applies own: what inrun sent. A field only envtest holds is a write the
// in-memory run lacks when an apply owns it, and otherwise an API-server
// default. A nil *owner knows nothing (no apply): such a field is taken as
// a default.
type owner struct {
	// node is the fieldsV1 entry for this position; nil when no manager
	// owns it.
	node map[string]any
	// all — a manager owns this position whole: everything below it, as
	// inside a string a manager wrote, was written, not defaulted.
	all bool
}

// managedOwner merges the fields u's server-side applies own into one
// tree. Only Apply entries count: the server defaults an object before it
// records a create, update or merge patch, so their entries own the
// defaults too, while an apply owns exactly the fields it sent. It returns
// nil when u has no Apply entry: without one, a field only envtest holds
// is taken as a default.
func managedOwner(u *unstructured.Unstructured) *owner {
	tree := map[string]any{}
	applied := false
	for _, e := range u.GetManagedFields() {
		if e.Operation != metav1.ManagedFieldsOperationApply || e.FieldsV1 == nil {
			continue
		}
		applied = true
		var fields map[string]any
		if json.Unmarshal(e.FieldsV1.Raw, &fields) == nil {
			mergeFields(tree, fields)
		}
	}
	if !applied {
		return nil
	}
	return &owner{node: tree}
}

func mergeFields(dst, src map[string]any) {
	for k, v := range src {
		sv, _ := v.(map[string]any)
		if dv, ok := dst[k].(map[string]any); ok && sv != nil {
			mergeFields(dv, sv)
			continue
		}
		if sv == nil {
			sv = map[string]any{}
		}
		dst[k] = sv
	}
}

// owned reports whether a manager wrote the position.
func (o *owner) owned() bool {
	return o != nil && (o.all || o.node != nil)
}

// key descends into map key k.
func (o *owner) key(k string) *owner {
	if o == nil || o.all {
		return o
	}
	child, _ := o.node["f:"+k].(map[string]any)
	return &owner{node: child}
}

// index descends into the list element el at i, found the way fieldsV1
// keys list elements: by its key fields (k:), by value (v:), or by index.
func (o *owner) index(i int, el any) *owner {
	if o == nil || o.all {
		return o
	}
	for k, v := range o.node {
		child, _ := v.(map[string]any)
		switch {
		case strings.HasPrefix(k, "k:"):
			var keyFields map[string]any
			m, isMap := el.(map[string]any)
			if isMap && json.Unmarshal([]byte(k[2:]), &keyFields) == nil && hasFields(m, keyFields) {
				return &owner{node: child}
			}
		case strings.HasPrefix(k, "v:"):
			var val any
			if json.Unmarshal([]byte(k[2:]), &val) == nil && reflect.DeepEqual(val, el) {
				return &owner{node: child}
			}
		case k == fmt.Sprintf("i:%d", i):
			return &owner{node: child}
		}
	}
	return &owner{}
}

// whole marks the position as owned with everything below it.
func (o *owner) whole() *owner {
	if !o.owned() {
		return o
	}
	return &owner{all: true}
}

func hasFields(m, fields map[string]any) bool {
	for k, v := range fields {
		if !reflect.DeepEqual(m[k], v) {
			return false
		}
	}
	return true
}

// diffValues walks memory and envtest together and returns each field that
// differs, with its path from the object root. own follows the walk through
// envtest's managedFields, to tell a default from a missing write.
func diffValues(path string, memory, envtest any, own *owner) []FieldDiff {
	// A map one run lacks is walked field by field: each field is a default,
	// a missing write or a pruned field on its own.
	if memory == nil {
		if _, ok := envtest.(map[string]any); ok {
			memory = map[string]any{}
		}
	}
	if envtest == nil {
		if _, ok := memory.(map[string]any); ok {
			envtest = map[string]any{}
		}
	}

	switch {
	case memory == nil && envtest == nil:
		return nil
	case memory == nil:
		if own.owned() {
			return []FieldDiff{{Path: path, Kind: FieldMissing, Envtest: envtest}}
		}
		return []FieldDiff{{Path: path, Kind: FieldDefaulted, Envtest: envtest}}
	case envtest == nil:
		return []FieldDiff{{Path: path, Kind: FieldPruned, Memory: memory}}
	}

	if mm, ok := memory.(map[string]any); ok {
		if em, ok := envtest.(map[string]any); ok {
			keys := map[string]bool{}
			for k := range mm {
				keys[k] = true
			}
			for k := range em {
				keys[k] = true
			}
			sorted := make([]string, 0, len(keys))
			for k := range keys {
				sorted = append(sorted, k)
			}
			sort.Strings(sorted)
			var out []FieldDiff
			for _, k := range sorted {
				out = append(out, diffValues(joinPath(path, k), mm[k], em[k], own.key(k))...)
			}
			return out
		}
	}

	if ml, ok := memory.([]any); ok {
		if el, ok := envtest.([]any); ok && len(ml) == len(el) {
			var out []FieldDiff
			for i := range ml {
				out = append(out, diffValues(fmt.Sprintf("%s[%d]", path, i), ml[i], el[i], own.index(i, el[i]))...)
			}
			return out
		}
	}

	if ms, ok := memory.(string); ok {
		if es, ok := envtest.(string); ok {
			// Both hold a time: when each run happened, not what it did.
			if isTime(ms) && isTime(es) {
				return nil
			}
			// A JSON document in a string (a health annotation) compares
			// field by field, so the rules here apply inside it. The API
			// server defaults nothing inside a string.
			if mj, ej, ok := embeddedJSON(ms, es); ok {
				return diffValues(path, mj, ej, own.whole())
			}
		}
	}

	if reflect.DeepEqual(memory, envtest) {
		return nil
	}
	return []FieldDiff{{Path: path, Kind: FieldChanged, Memory: memory, Envtest: envtest}}
}

// isTime reports whether s is an RFC 3339 time, as Kubernetes writes them.
func isTime(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

// embeddedJSON decodes two strings that each hold a JSON object or array.
func embeddedJSON(a, b string) (any, any, bool) {
	decode := func(s string) (any, bool) {
		s = strings.TrimSpace(s)
		if s == "" || (s[0] != '{' && s[0] != '[') {
			return nil, false
		}
		var v any
		if json.Unmarshal([]byte(s), &v) != nil {
			return nil, false
		}
		return dropEmpty(v), true
	}
	av, aok := decode(a)
	bv, bok := decode(b)
	return av, bv, aok && bok
}

// joinPath appends key to path: dotted, or bracketed when the key itself
// holds a dot or slash, as label and annotation keys do.
func joinPath(path, key string) string {
	if strings.ContainsAny(key, "./") {
		return fmt.Sprintf("%s[%q]", path, key)
	}
	if path == "" {
		return key
	}
	return path + "." + key
}
