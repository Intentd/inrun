package template

import (
	"fmt"
	"reflect"

	"github.com/intentd/inrun/pkg/template/funcs"
	"github.com/intentd/inrun/pkg/types"
	"github.com/intentd/inrun/pkg/util/structured"
)

// NestedOption changes how ResolveNested treats a value.
type NestedOption func(*nestedOpts)

type nestedOpts struct{ keepOnError, dropEmpty bool }

// KeepOnError keeps a string that fails to resolve as written, instead of
// returning the error.
func KeepOnError() NestedOption { return func(o *nestedOpts) { o.keepOnError = true } }

// DropEmpty turns a template that resolves to "" into nil, so the field is
// removed. Without it the template is kept as written.
func DropEmpty() NestedOption { return func(o *nestedOpts) { o.dropEmpty = true } }

// ResolveNested resolves every string in in, recursing into maps and slices,
// and returns a new map. A resolved template is coerced to its native type
// (number, bool, JSON), and a function's JSON-value sentinel is decoded, so
// typed CRD fields pass API validation. Plain strings and string-map values
// stay strings. The first failure is returned with its path.
func (r *Resolver) ResolveNested(in map[string]any, opts ...NestedOption) (map[string]any, error) {
	var o nestedOpts
	for _, opt := range opts {
		opt(&o)
	}
	return resolveMap(r, in, o)
}

// ResolveNestedLenient is ResolveNested with KeepOnError.
func (r *Resolver) ResolveNestedLenient(in map[string]any) map[string]any {
	out, _ := r.ResolveNested(in, KeepOnError())
	return out
}

func resolveMap(r *Resolver, in map[string]any, o nestedOpts) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k, v := range in {
		rv, err := resolveValue(r, v, o)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = rv
	}
	return out, nil
}

func resolveValue(r *Resolver, v any, o nestedOpts) (any, error) {
	switch vv := v.(type) {
	case string:
		return resolveLeaf(r, vv, o)
	case map[string]any:
		return resolveMap(r, vv, o)
	case []any:
		out := make([]any, 0, len(vv))
		for i, e := range vv {
			re, err := resolveValue(r, e, o)
			if err != nil {
				return nil, fmt.Errorf("[%d]: %w", i, err)
			}
			out = append(out, re)
		}
		return out, nil
	}

	// A string-keyed, string-valued map of any named type: resolve values,
	// without coercion.
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Map || rv.Type().Key().Kind() != reflect.String || rv.Type().Elem().Kind() != reflect.String {
		return v, nil
	}
	out := make(map[string]any, rv.Len())
	for _, key := range rv.MapKeys() {
		s := rv.MapIndex(key).String()
		res, err := r.Resolve(s)
		switch {
		case err != nil && o.keepOnError:
			res = s
		case err != nil:
			return nil, fmt.Errorf("%s: %w", key.String(), err)
		case res == "":
			res = s
		}
		out[key.String()] = res
	}
	return out, nil
}

// resolveLeaf resolves one string value.
func resolveLeaf(r *Resolver, s string, o nestedOpts) (any, error) {
	res, err := r.Resolve(s)
	if err != nil {
		if o.keepOnError {
			return s, nil
		}
		return nil, err
	}
	if funcs.HasJSONValueSentinel(res) {
		var v any
		if err := funcs.DecodeJSONValueSentinel(res, &v); err != nil {
			return nil, fmt.Errorf("invalid JSON payload: %w", err)
		}
		return v, nil
	}
	if res == "" {
		if o.dropEmpty && types.IsTemplate(s) {
			return nil, nil
		}
		return s, nil
	}
	if types.IsTemplate(s) {
		return structured.TryCoerceString(res), nil
	}
	return res, nil
}
