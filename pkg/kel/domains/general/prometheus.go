package note

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"
)

// Prometheus FuncMap notes.
// All functions accept the map injected under .external.<name> by the prometheus client.
//
//	{{ promValue .external.errorRate }}                   → scalar or first-series value
//	{{ promSum .external.errorRate }}                     → sum of all vector series values
//	{{ promMax .external.errorRate }}                     → max across all series
//	{{ promAboveThreshold .external.queueDepth 1000 }}    → "true" / "false"
//	{{ promSeriesCount .external.activeInstances }}       → number of series as string
//	{{ promLabelValues .external.pods "namespace" }}      → comma-separated label values

func prometheusNotes() template.FuncMap {
	return template.FuncMap{
		"promValue":          notePromValue,
		"promSum":            notePromSum,
		"promMax":            notePromMax,
		"promAboveThreshold": notePromAboveThreshold,
		"promBelowThreshold": notePromBelowThreshold,
		"promSeriesCount":    notePromSeriesCount,
		"promLabelValues":    notePromLabelValues,
	}
}

// notePromValue returns the canonical scalar result of a Prometheus instant query.
// For scalar results: the value. For vector results: the first series value.
func notePromValue(data interface{}) string {
	m, ok := data.(map[string]interface{})
	if !ok {
		return ""
	}
	result, _ := m["result"].(string)
	return result
}

// notePromSum sums all series values in a vector result.
// For scalar results, returns the value unchanged.
func notePromSum(data interface{}) string {
	vals := promExtractValues(data)
	if len(vals) == 0 {
		return ""
	}
	var sum float64
	for _, v := range vals {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return ""
		}
		sum += f
	}
	return formatFloat(sum)
}

// notePromMax returns the maximum series value in a vector result.
func notePromMax(data interface{}) string {
	vals := promExtractValues(data)
	if len(vals) == 0 {
		return ""
	}
	var max float64
	first := true
	for _, v := range vals {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return ""
		}
		if first || f > max {
			max = f
			first = false
		}
	}
	return formatFloat(max)
}

// notePromBelowThreshold returns "true" when promValue < threshold.
func notePromBelowThreshold(data interface{}, threshold interface{}) string {
	val := notePromValue(data)
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return "false"
	}
	var t float64
	switch v := threshold.(type) {
	case int:
		t = float64(v)
	case int64:
		t = float64(v)
	case float64:
		t = v
	case string:
		if t, err = strconv.ParseFloat(v, 64); err != nil {
			return "false"
		}
	default:
		t, err = strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		if err != nil {
			return "false"
		}
	}
	if f < t {
		return "true"
	}
	return "false"
}

// notePromAboveThreshold returns "true" when promValue > threshold.
func notePromAboveThreshold(data interface{}, threshold interface{}) string {
	val := notePromValue(data)
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return "false"
	}
	var t float64
	switch v := threshold.(type) {
	case int:
		t = float64(v)
	case int64:
		t = float64(v)
	case float64:
		t = v
	case string:
		if t, err = strconv.ParseFloat(v, 64); err != nil {
			return "false"
		}
	default:
		t, err = strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		if err != nil {
			return "false"
		}
	}
	if f > t {
		return "true"
	}
	return "false"
}

// notePromSeriesCount returns the number of series in a vector result as a string.
func notePromSeriesCount(data interface{}) string {
	vals := promExtractValues(data)
	return strconv.Itoa(len(vals))
}

// notePromLabelValues returns comma-separated values of the given label across all series.
func notePromLabelValues(data interface{}, label string) string {
	m, ok := data.(map[string]interface{})
	if !ok {
		return ""
	}
	raw, ok := m["raw"].(map[string]interface{})
	if !ok {
		return ""
	}
	d, ok := raw["data"].(map[string]interface{})
	if !ok {
		return ""
	}
	result, ok := d["result"].([]interface{})
	if !ok {
		return ""
	}
	var vals []string
	for _, r := range result {
		series, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		metric, ok := series["metric"].(map[string]interface{})
		if !ok {
			continue
		}
		if v, ok := metric[label].(string); ok {
			vals = append(vals, v)
		}
	}
	return strings.Join(vals, ",")
}

// promExtractValues pulls all series values from the raw Prometheus response.
func promExtractValues(data interface{}) []string {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := m["raw"].(map[string]interface{})
	if !ok {
		// Fall back to .result for scalar
		if r, ok := m["result"].(string); ok && r != "" {
			return []string{r}
		}
		return nil
	}
	d, ok := raw["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	switch d["resultType"] {
	case "scalar":
		result, ok := d["result"].([]interface{})
		if !ok || len(result) < 2 {
			return nil
		}
		v, ok := result[1].(string)
		if !ok {
			return nil
		}
		return []string{v}

	case "vector":
		result, ok := d["result"].([]interface{})
		if !ok {
			return nil
		}
		var vals []string
		for _, r := range result {
			series, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			value, ok := series["value"].([]interface{})
			if !ok || len(value) < 2 {
				continue
			}
			if v, ok := value[1].(string); ok {
				vals = append(vals, v)
			}
		}
		return vals
	}
	return nil
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
