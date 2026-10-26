package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Args are the values a reconciler receives on each reconcile, declared in the
// Blueprint under operatorBox.reconcile.args. Template expressions are rendered
// against the CR first, so args express the CR in the reconciler's own
// vocabulary. The runtime puts them on Request.Args for every reconciler,
// hooks included.
//
// Values are YAML scalars (string, int, bool), maps or slices. Accessors convert
// on the fly and return zero values when the key is absent or the wrong type,
// so treat an absent arg as "use the default".
type Args map[string]interface{}

// String returns the value for key as a string. Scalars are formatted, so a
// template that renders "true" or "3" (coerced to bool or int) still reads
// back as "true" or "3". Returns "" if the key is absent, a map or a slice.
func (a Args) String(key string) string {
	switch v := a[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case bool, int, int64, float64:
		return fmt.Sprint(v)
	default:
		return ""
	}
}

// Bool returns the value for key as a bool. A string is parsed with
// strconv.ParseBool. Returns false if absent or not a boolean value.
func (a Args) Bool(key string) bool {
	switch v := a[key].(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	default:
		return false
	}
}

// Int returns the value for key as an int.
// YAML unmarshals integers as int, int64, or float64 depending on the decoder;
// all three are tried. Template-evaluated args arrive as strings — those are
// parsed with strconv so {{ default "3" .spec.replicas }} works naturally.
func (a Args) Int(key string) int {
	switch v := a[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// Sub returns the nested map at key as an Args value.
// Returns an empty Args if the key is absent or is not a map.
func (a Args) Sub(key string) Args {
	switch v := a[key].(type) {
	case map[string]interface{}:
		return Args(v)
	case Args:
		return v
	}
	return Args{}
}

// Slice returns the slice at key as []interface{}.
// Returns nil if the key is absent or is not a slice.
func (a Args) Slice(key string) []interface{} {
	v, _ := a[key].([]interface{})
	return v
}

// Bind JSON-round-trips the args into dst, which must point to a struct.
// Use it for a typed args struct in your hook or reconciler.
//
//	type MyArgs struct {
//	    Region string `json:"region"`
//	}
//	var cfg MyArgs
//	if err := req.Args().Bind(&cfg); err != nil { ... }
func (a Args) Bind(dst interface{}) error {
	b, err := json.Marshal(map[string]interface{}(a))
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
