package types

import (
	"os"
	"reflect"
	"sort"
	"strings"
)

// ExpandEnv returns a copy of v with ${NAME} expanded through env in every
// string it holds, the way the template resolver resolves a source before it
// is used: an expectation is resolved once, and the code that runs it sees
// only final values. Fields tagged env:"-" keep their text — a command's run,
// which its shell expands from the process environment, and an expectation's
// own env, which is where the values come from. v is not modified.
func ExpandEnv[T any](env EnvResolver, v T) T {
	out := walkEnv(reflect.ValueOf(&v).Elem(), env.Expand)
	return out.Interface().(T)
}

// UnknownEnv returns the ${NAME} references in v, outside fields tagged
// env:"-", that env cannot resolve: declared nowhere and unset in the process
// environment. Sorted, each name once.
func UnknownEnv(env EnvResolver, v any) []string {
	seen := map[string]bool{}
	walkEnv(reflect.ValueOf(v), func(s string) string {
		for _, m := range envRef.FindAllStringSubmatch(s, -1) {
			if _, ok := env.LookupEnv(m[1]); !ok {
				seen[m[1]] = true
			}
		}
		return s
	})
	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// Map returns every value env resolves: the process environment overlaid by
// the declared env. Templates see it as .env.
func (e EnvResolver) Map() map[string]string {
	out := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			out[k] = v
		}
	}
	for k, v := range e.values {
		out[k] = v
	}
	return out
}

// walkEnv returns a deep copy of v with fn applied to every string, skipping
// struct fields tagged env:"-". Map keys are kept as written.
func walkEnv(v reflect.Value, fn func(string) string) reflect.Value {
	switch v.Kind() {
	case reflect.String:
		out := reflect.New(v.Type()).Elem()
		out.SetString(fn(v.String()))
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() || f.Tag.Get("env") == "-" {
				continue
			}
			out.Field(i).Set(walkEnv(v.Field(i), fn))
		}
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(walkEnv(v.Elem(), fn))
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(walkEnv(v.Index(i), fn))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		for iter := v.MapRange(); iter.Next(); {
			out.SetMapIndex(iter.Key(), walkEnv(iter.Value(), fn))
		}
		return out
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(walkEnv(v.Elem(), fn))
		return out
	}
	return v
}
