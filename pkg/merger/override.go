package merger

import (
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/structured"
)

// Set is one override of a merged CRD entry for a single run, such as
// `--set migrate.prune=true`: a dot path through the entry's own YAML keys and
// a value read as YAML, so true, 3 and text keep their types.
type Set struct {
	Path  string
	Value string
}

func (s Set) String() string { return s.Path + "=" + s.Value }

// ParseSets reads key=value arguments. The value is everything after the
// first "=", so it may contain "=" itself (e.g. selection.selector=tier=web).
func ParseSets(args []string) ([]Set, error) {
	sets := make([]Set, 0, len(args))
	for _, a := range args {
		path, value, ok := strings.Cut(a, "=")
		path = strings.TrimSpace(path)
		if !ok || path == "" {
			return nil, fmt.Errorf("--set %q: want key=value", a)
		}
		for _, part := range strings.Split(path, ".") {
			if part == "" {
				return nil, fmt.Errorf("--set %q: empty key in path %q", a, path)
			}
		}
		sets = append(sets, Set{Path: path, Value: value})
	}
	return sets, nil
}

// Override applies sets to the merged CRD entry name, after Merge and before
// the Blueprint is built and validated, so an override is checked like the
// file it overrides. A path the entry's schema doesn't know is an error, never
// ignored. Fields that are not part of the YAML (resolved by the merger) are
// kept as they were.
func (m *Merger) Override(name string, sets []Set) error {
	m.mustBeMerged()
	entry, ok := m.result[name]
	if !ok {
		return fmt.Errorf("--set: no CRD entry %q", name)
	}
	if len(sets) == 0 {
		return nil
	}

	raw, err := yaml.Marshal(entry)
	if err != nil {
		return fmt.Errorf("--set: encoding %s: %w", name, err)
	}
	doc := map[string]any{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("--set: decoding %s: %w", name, err)
	}
	for _, s := range sets {
		var value any
		if err := yaml.Unmarshal([]byte(s.Value), &value); err != nil {
			return fmt.Errorf("--set %s: value is not valid YAML: %w", s, err)
		}
		if err := setPath(doc, strings.Split(s.Path, "."), value); err != nil {
			return fmt.Errorf("--set %s: %w", s, err)
		}
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("--set: encoding %s: %w", name, err)
	}
	var updated types.CRDEntry
	if err := structured.StrictUnmarshal(out, &updated); err != nil {
		return fmt.Errorf("--set on %s: %s", name, err)
	}
	keepUnserialized(&updated, &entry)
	m.result[name] = updated
	return nil
}

// setPath sets path in doc to value, creating the maps along the way. It
// refuses to go through a value that is not a map (a list or a scalar).
func setPath(doc map[string]any, path []string, value any) error {
	cur := doc
	for i, key := range path[:len(path)-1] {
		next, ok := cur[key]
		if !ok || next == nil {
			child := map[string]any{}
			cur[key] = child
			cur = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("%s is not a map", strings.Join(path[:i+1], "."))
		}
		cur = child
	}
	cur[path[len(path)-1]] = value
	return nil
}

// keepUnserialized copies the fields tagged yaml:"-" from orig into updated:
// the YAML round trip drops them, and they are the merger's, not the user's.
func keepUnserialized(updated, orig *types.CRDEntry) {
	uv, ov := reflect.ValueOf(updated).Elem(), reflect.ValueOf(orig).Elem()
	t := uv.Type()
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("yaml") == "-" && uv.Field(i).CanSet() {
			uv.Field(i).Set(ov.Field(i))
		}
	}
}
