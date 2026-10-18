package utils

import "encoding/json"

// JSONNormalize round-trips a value through encoding/json, normalising all
// numeric types to float64. This ensures values from the Kubernetes API
// (int64 for integer CRD fields) compare correctly with values decoded from
// annotation JSON strings (float64 via encoding/json).
func JSONNormalize(v interface{}) interface{} {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out interface{}
	err = json.Unmarshal(b, &out)
	if err != nil {
		return v
	}
	return out
}
