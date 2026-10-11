package children

import (
	"context"

	"github.com/intentd/inrun/pkg/kubeclient"
	"github.com/intentd/inrun/pkg/types"
)

// enrichGroupWithHPAData embeds current metrics and scale target info for each
// HorizontalPodAutoscaler in the group. A no-op when hpa enrichment is not
// enabled on the CRD.
//
// _currentMetrics: reshaped from status.currentMetrics — each entry has
//
//	{type, name, current, target}.
//
// _scaleTarget: from spec.scaleTargetRef — {name, kind, apiVersion}.
func enrichGroupWithHPAData(_ context.Context, _ kubeclient.Interface, m map[string]interface{}, crd types.CRDEntry) {
	if !enrichmentEnabled("hpa", crd) {
		return
	}

	for _, v := range m {
		obj, ok := v.(map[string]interface{})
		if !ok {
			continue
		}

		// _scaleTarget from spec.scaleTargetRef
		spec, _ := obj["spec"].(map[string]interface{})
		if spec != nil {
			ref, _ := spec["scaleTargetRef"].(map[string]interface{})
			if ref != nil {
				obj["_scaleTarget"] = map[string]interface{}{
					"name":       ref["name"],
					"kind":       ref["kind"],
					"apiVersion": ref["apiVersion"],
				}
			}
		}

		// _currentMetrics from status.currentMetrics
		status, _ := obj["status"].(map[string]interface{})
		if status == nil {
			continue
		}
		currentMetrics, _ := status["currentMetrics"].([]interface{})
		if len(currentMetrics) == 0 {
			continue
		}
		reshaped := make([]interface{}, 0, len(currentMetrics))
		for _, cm := range currentMetrics {
			cmMap, _ := cm.(map[string]interface{})
			if cmMap == nil {
				continue
			}
			reshaped = append(reshaped, reshapeMetric(cmMap))
		}
		if len(reshaped) > 0 {
			obj["_currentMetrics"] = reshaped
		}
	}
}

// reshapeMetric extracts the essential fields from a currentMetric entry,
// normalising across the four metric source types (Resource, External, Pods, Object).
func reshapeMetric(m map[string]interface{}) map[string]interface{} {
	metricType, _ := m["type"].(string)
	result := map[string]interface{}{"type": metricType}

	switch metricType {
	case "Resource":
		if r, _ := m["resource"].(map[string]interface{}); r != nil {
			result["name"] = r["name"]
			if cur, _ := r["current"].(map[string]interface{}); cur != nil {
				result["current"] = cur["averageUtilization"]
				if result["current"] == nil {
					result["current"] = cur["averageValue"]
				}
			}
		}
	case "External":
		if e, _ := m["external"].(map[string]interface{}); e != nil {
			if metric, _ := e["metric"].(map[string]interface{}); metric != nil {
				result["name"] = metric["name"]
			}
			if cur, _ := e["current"].(map[string]interface{}); cur != nil {
				result["current"] = cur["value"]
				if result["current"] == nil {
					result["current"] = cur["averageValue"]
				}
			}
		}
	case "Pods":
		if p, _ := m["pods"].(map[string]interface{}); p != nil {
			if metric, _ := p["metric"].(map[string]interface{}); metric != nil {
				result["name"] = metric["name"]
			}
			if cur, _ := p["current"].(map[string]interface{}); cur != nil {
				result["current"] = cur["averageValue"]
			}
		}
	case "Object":
		if o, _ := m["object"].(map[string]interface{}); o != nil {
			if metric, _ := o["metric"].(map[string]interface{}); metric != nil {
				result["name"] = metric["name"]
			}
			if cur, _ := o["current"].(map[string]interface{}); cur != nil {
				result["current"] = cur["value"]
				if result["current"] == nil {
					result["current"] = cur["averageValue"]
				}
			}
		}
	}
	return result
}
