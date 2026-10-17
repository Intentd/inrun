package note

import (
	"fmt"
	"text/template"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Resource quantity notes for Kubernetes-style CPU and memory math.
//
// Kubernetes quantities use SI suffixes (m, K, M, G, T, P, E) and binary
// suffixes (Ki, Mi, Gi, Ti, Pi, Ei). These notes let Catalog authors do
// resource budget arithmetic without writing Go:
//
//	"give each tenant 1/N of available capacity"
//	"cap a deployment's CPU request at cluster node size × headroom"
//
// Usage:
//
//	{{ parseQuantity "100m" }}                    →  0.1
//	{{ formatQuantity 0.1 }}                      →  "100m"
//	{{ sumQuantity "100m" "200m" }}               →  "300m"
//	{{ sumQuantity "1Gi" "512Mi" }}               →  "1536Mi"
//	{{ formatQuantity (div (parseQuantity .spec.cpuLimit) 4) }}   →  "250m"

func quantityNotes() template.FuncMap {
	return template.FuncMap{
		"parseQuantity":  noteParseQuantity,
		"formatQuantity": noteFormatQuantity,
		"sumQuantity":    noteSumQuantity,
		"minusQuantity":  noteSubtractQuantity,
	}
}

// noteParseQuantity converts a Kubernetes quantity string to float64.
// CPU quantities use milli-cores (m suffix): "100m" → 0.1, "1" → 1.0.
// Memory quantities use binary suffixes: "1Gi" → 1073741824.0.
// Returns an error when the string is not a valid quantity.
//
//	{{ parseQuantity "100m" }}    →  0.1
//	{{ parseQuantity "500m" }}    →  0.5
//	{{ parseQuantity "2" }}       →  2.0
//	{{ parseQuantity "1Gi" }}     →  1073741824.0
func noteParseQuantity(q string) (float64, error) {
	parsed, err := resource.ParseQuantity(q)
	if err != nil {
		return 0, fmt.Errorf("parseQuantity: %q is not a valid Kubernetes quantity: %w", q, err)
	}
	return parsed.AsApproximateFloat64(), nil
}

// noteFormatQuantity converts a float64 back to a canonical Kubernetes
// quantity string. CPU fractions are expressed in milli-cores (m suffix).
// Memory values are expressed in whole bytes or binary suffixes.
//
//	{{ formatQuantity 0.1 }}       →  "100m"
//	{{ formatQuantity 0.5 }}       →  "500m"
//	{{ formatQuantity 1.0 }}       →  "1"
//	{{ formatQuantity 1073741824 }} →  "1Gi"
func noteFormatQuantity(f float64) (string, error) {
	// Check if the value is an integer and >= 1 — likely a memory quantity
	if f == float64(int64(f)) && f >= 1 {
		q := resource.NewQuantity(int64(f), resource.BinarySI)
		return q.String(), nil
	}
	// Otherwise treat as CPU (milli‑units)
	millis := int64(f * 1000)
	q := resource.NewMilliQuantity(millis, resource.DecimalSI)
	return q.String(), nil
}

// noteSumQuantity adds two Kubernetes quantity strings and returns the
// canonical string representation of their sum.
//
//	{{ sumQuantity "100m" "200m" }}   →  "300m"
//	{{ sumQuantity "500m" "500m" }}   →  "1"
//	{{ sumQuantity "1Gi" "512Mi" }}   →  "1536Mi"
func noteSumQuantity(a, b string) (string, error) {
	qa, err := resource.ParseQuantity(a)
	if err != nil {
		return "", fmt.Errorf("sumQuantity: %q: %w", a, err)
	}
	qb, err := resource.ParseQuantity(b)
	if err != nil {
		return "", fmt.Errorf("sumQuantity: %q: %w", b, err)
	}
	qa.Add(qb)
	return qa.String(), nil
}

// noteSubtractQuantity subtracts the second quantity from the first and returns
// the canonical string representation of their difference.
//
//	{{ subtractQuantity "300m" "100m" }}   →  "200m"
//	{{ subtractQuantity "1" "500m" }}      →  "500m"
//	{{ subtractQuantity "1536Mi" "512Mi" }} →  "1024Mi"
func noteSubtractQuantity(a, b string) (string, error) {
	qa, err := resource.ParseQuantity(a)
	if err != nil {
		return "", fmt.Errorf("subtractQuantity: %q: %w", a, err)
	}
	qb, err := resource.ParseQuantity(b)
	if err != nil {
		return "", fmt.Errorf("subtractQuantity: %q: %w", b, err)
	}
	qa.Sub(qb)
	return qa.String(), nil
}
