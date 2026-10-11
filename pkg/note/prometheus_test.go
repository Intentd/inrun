package note_test

import (
	"testing"

	"github.com/intentd/inrun/pkg/note"
)

func vectorData(series []map[string]interface{}) map[string]interface{} {
	result := make([]interface{}, len(series))
	for i, s := range series {
		result[i] = map[string]interface{}{
			"metric": s["metric"],
			"value":  []interface{}{"1753228800", s["value"]},
		}
	}
	return map[string]interface{}{
		"called": "true",
		"result": series[0]["value"],
		"raw": map[string]interface{}{
			"data": map[string]interface{}{
				"resultType": "vector",
				"result":     result,
			},
		},
	}
}

func scalarData(value string) map[string]interface{} {
	return map[string]interface{}{
		"called": "true",
		"result": value,
		"raw": map[string]interface{}{
			"data": map[string]interface{}{
				"resultType": "scalar",
				"result":     []interface{}{"1753228800", value},
			},
		},
	}
}

func TestPromValue(t *testing.T) {
	m := note.Map()

	fn := m["promValue"].(func(interface{}) string)

	if got := fn(nil); got != "" {
		t.Errorf("nil: want empty, got %q", got)
	}
	if got := fn(vectorData([]map[string]interface{}{{"metric": map[string]interface{}{}, "value": "42"}})); got != "42" {
		t.Errorf("vector: want 42, got %q", got)
	}
	if got := fn(scalarData("3.14")); got != "3.14" {
		t.Errorf("scalar: want 3.14, got %q", got)
	}
}

func TestPromSum(t *testing.T) {
	m := note.Map()
	fn := m["promSum"].(func(interface{}) string)

	if got := fn(nil); got != "" {
		t.Errorf("nil: want empty, got %q", got)
	}

	data := vectorData([]map[string]interface{}{
		{"metric": map[string]interface{}{}, "value": "10"},
		{"metric": map[string]interface{}{}, "value": "20"},
		{"metric": map[string]interface{}{}, "value": "30"},
	})
	// fix: all series need individual value entries — rebuild manually
	data["raw"].(map[string]interface{})["data"].(map[string]interface{})["result"] = []interface{}{
		map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "10"}},
		map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "20"}},
		map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "30"}},
	}
	if got := fn(data); got != "60" {
		t.Errorf("sum: want 60, got %q", got)
	}
}

func TestPromMax(t *testing.T) {
	m := note.Map()
	fn := m["promMax"].(func(interface{}) string)

	if got := fn(nil); got != "" {
		t.Errorf("nil: want empty, got %q", got)
	}

	data := map[string]interface{}{
		"result": "5",
		"raw": map[string]interface{}{
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []interface{}{
					map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "5"}},
					map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "99"}},
					map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "12"}},
				},
			},
		},
	}
	if got := fn(data); got != "99" {
		t.Errorf("max: want 99, got %q", got)
	}
}

func TestPromAboveThreshold(t *testing.T) {
	m := note.Map()
	fn := m["promAboveThreshold"].(func(interface{}, interface{}) string)

	if got := fn(nil, 0); got != "false" {
		t.Errorf("nil: want false, got %q", got)
	}

	above := vectorData([]map[string]interface{}{{"metric": map[string]interface{}{}, "value": "42"}})
	if got := fn(above, 10); got != "true" {
		t.Errorf("above int: want true, got %q", got)
	}
	if got := fn(above, 100); got != "false" {
		t.Errorf("below int: want false, got %q", got)
	}
	if got := fn(above, "42"); got != "false" {
		t.Errorf("equal string: want false (strict >), got %q", got)
	}
	if got := fn(above, 42.0); got != "false" {
		t.Errorf("equal float: want false (strict >), got %q", got)
	}
}

func TestPromBelowThreshold(t *testing.T) {
	m := note.Map()
	fn := m["promBelowThreshold"].(func(interface{}, interface{}) string)

	if got := fn(nil, 0); got != "false" {
		t.Errorf("nil: want false, got %q", got)
	}

	data := vectorData([]map[string]interface{}{{"metric": map[string]interface{}{}, "value": "5"}})
	if got := fn(data, 10); got != "true" {
		t.Errorf("below int: want true, got %q", got)
	}
	if got := fn(data, 1); got != "false" {
		t.Errorf("above int: want false, got %q", got)
	}
	if got := fn(data, "5"); got != "false" {
		t.Errorf("equal string: want false (strict <), got %q", got)
	}
}

func TestPromSeriesCount(t *testing.T) {
	m := note.Map()
	fn := m["promSeriesCount"].(func(interface{}) string)

	if got := fn(nil); got != "0" {
		t.Errorf("nil: want 0, got %q", got)
	}

	data := map[string]interface{}{
		"result": "1",
		"raw": map[string]interface{}{
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []interface{}{
					map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "1"}},
					map[string]interface{}{"metric": map[string]interface{}{}, "value": []interface{}{"t", "2"}},
				},
			},
		},
	}
	if got := fn(data); got != "2" {
		t.Errorf("two series: want 2, got %q", got)
	}
}

func TestPromLabelValues(t *testing.T) {
	m := note.Map()
	fn := m["promLabelValues"].(func(interface{}, string) string)

	if got := fn(nil, "ns"); got != "" {
		t.Errorf("nil: want empty, got %q", got)
	}

	data := map[string]interface{}{
		"result": "1",
		"raw": map[string]interface{}{
			"data": map[string]interface{}{
				"resultType": "vector",
				"result": []interface{}{
					map[string]interface{}{"metric": map[string]interface{}{"namespace": "default"}, "value": []interface{}{"t", "1"}},
					map[string]interface{}{"metric": map[string]interface{}{"namespace": "kube-system"}, "value": []interface{}{"t", "2"}},
				},
			},
		},
	}
	if got := fn(data, "namespace"); got != "default,kube-system" {
		t.Errorf("label values: want default,kube-system, got %q", got)
	}
	if got := fn(data, "missing"); got != "" {
		t.Errorf("missing label: want empty, got %q", got)
	}
}
