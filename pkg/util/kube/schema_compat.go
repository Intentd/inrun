package kube

import (
	"fmt"
	"sort"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
)

// SchemaLeafPaths returns every leaf under schema as dotted path → type.
// A leaf is a non-object, or an object with no declared properties (a map or
// a preserve-unknown-fields object). Array items that are objects are walked
// with "[]" appended to the array's path.
func SchemaLeafPaths(schema *apiextensions.JSONSchemaProps) map[string]string {
	out := map[string]string{}
	if schema != nil {
		walkLeaves("", schema, out)
	}
	return out
}

func walkLeaves(path string, s *apiextensions.JSONSchemaProps, out map[string]string) {
	switch {
	case len(s.Properties) > 0:
		for name, child := range s.Properties {
			child := child
			walkLeaves(join(path, name), &child, out)
		}
	case s.Type == "array" && s.Items != nil && s.Items.Schema != nil && len(s.Items.Schema.Properties) > 0:
		walkLeaves(path+"[]", s.Items.Schema, out)
	case path != "":
		out[path] = schemaType(s)
	}
}

// CompareSchemas returns, sorted, each leaf of src that dst cannot hold: the
// path is missing, or its type differs. A dst object that preserves unknown
// fields, or a map (additionalProperties), holds anything below it. An
// integer fits a number.
func CompareSchemas(src, dst *apiextensions.JSONSchemaProps) []string {
	var problems []string
	for path, typ := range SchemaLeafPaths(src) {
		if msg := holds(dst, path, typ); msg != "" {
			problems = append(problems, fmt.Sprintf("%s: %s", path, msg))
		}
	}
	sort.Strings(problems)
	return problems
}

// holds reports why dst cannot hold a value of typ at path, or "".
func holds(dst *apiextensions.JSONSchemaProps, path, typ string) string {
	cur := dst
	for _, seg := range splitPath(path) {
		if cur == nil {
			return "missing in destination"
		}
		if opensAnything(cur) {
			return ""
		}
		if seg == "[]" {
			if cur.Type != "array" || cur.Items == nil || cur.Items.Schema == nil {
				return fmt.Sprintf("destination is %s, not an array", schemaType(cur))
			}
			cur = cur.Items.Schema
			continue
		}
		next, ok := cur.Properties[seg]
		if !ok {
			return "missing in destination"
		}
		cur = &next
	}
	if opensAnything(cur) && len(cur.Properties) == 0 && (typ == "object" || typ == "any") {
		return ""
	}
	got := schemaType(cur)
	if got == typ || got == "any" || (got == "number" && typ == "integer") {
		return ""
	}
	return fmt.Sprintf("source is %s, destination is %s", typ, got)
}

// opensAnything reports whether s accepts fields it does not declare.
func opensAnything(s *apiextensions.JSONSchemaProps) bool {
	if s.XPreserveUnknownFields != nil && *s.XPreserveUnknownFields {
		return true
	}
	return s.AdditionalProperties != nil && (s.AdditionalProperties.Allows || s.AdditionalProperties.Schema != nil)
}

func schemaType(s *apiextensions.JSONSchemaProps) string {
	switch {
	case s.Type != "":
		return s.Type
	case s.XIntOrString:
		return "int-or-string"
	default:
		return "any"
	}
}

func join(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// splitPath splits "a.b[].c" into a, b, [], c.
func splitPath(path string) []string {
	var out []string
	cur := ""
	for i := 0; i < len(path); i++ {
		switch {
		case path[i] == '.':
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
		case path[i] == '[' && i+1 < len(path) && path[i+1] == ']':
			if cur != "" {
				out = append(out, cur)
			}
			out = append(out, "[]")
			cur = ""
			i++
		default:
			cur += string(path[i])
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
