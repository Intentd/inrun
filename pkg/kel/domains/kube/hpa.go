package note

import "text/template"

// hpaNotes registers helpers for inspecting HorizontalPodAutoscaler status fields.
//
// Usage:
//
//	tmpl.Funcs(note.hpaNotes())
//
// Template examples:
//
//	{{ hpaCurrentReplicas .children.hpa }}
//	{{ hpaDesiredReplicas .children.hpa }}
//	{{ hpaMinReplicas .children.hpa }}
//	{{ hpaMaxReplicas .children.hpa }}
//	{{ hpaScaling .children.hpa }}
//	{{ hpaAtMax .children.hpa }}
//
// No enrichment required — all notes navigate the HPA object directly.
func hpaNotes() template.FuncMap {
	return template.FuncMap{
		"hpaCurrentReplicas": noteHPACurrentReplicas,
		"hpaDesiredReplicas": noteHPADesiredReplicas,
		"hpaMinReplicas":     noteHPAMinReplicas,
		"hpaMaxReplicas":     noteHPAMaxReplicas,
		"hpaScaling":         noteHPAScaling,
		"hpaAtMax":           noteHPAAtMax,
		// HPA condition notes — read from status.conditions.
		"hpaScalingActive":  noteHPAScalingActive,
		"hpaAbleToScale":    noteHPAAbleToScale,
		"hpaScalingLimited": noteHPAScalingLimited,
		// Enriched scale-target notes — require enrich: [hpa] on the CRD.
		"hpaScaleTargetName": noteHPAScaleTargetName,
		"hpaScaleTargetKind": noteHPAScaleTargetKind,
		"hpaMetricTypes":     noteHPAMetricTypes,
	}
}

// ── HPA notes ─────────────────────────────────────────────────────────────────

// noteHPACurrentReplicas returns status.currentReplicas.
//
//	{{ hpaCurrentReplicas .children.hpa }}  → 3
func noteHPACurrentReplicas(obj interface{}) int64 {
	status := noteStatus(obj)
	return toInt64(status["currentReplicas"])
}

// noteHPADesiredReplicas returns status.desiredReplicas.
//
//	{{ hpaDesiredReplicas .children.hpa }}  → 5
func noteHPADesiredReplicas(obj interface{}) int64 {
	status := noteStatus(obj)
	return toInt64(status["desiredReplicas"])
}

// noteHPAMinReplicas returns spec.minReplicas.
// Returns 1 when not set (Kubernetes default).
//
//	{{ hpaMinReplicas .children.hpa }}  → 2
func noteHPAMinReplicas(obj interface{}) int64 {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return 1
	}
	spec, _ := m["spec"].(map[string]interface{})
	if spec == nil {
		return 1
	}
	v := toInt64(spec["minReplicas"])
	if v == 0 {
		return 1
	}
	return v
}

// noteHPAMaxReplicas returns spec.maxReplicas.
//
//	{{ hpaMaxReplicas .children.hpa }}  → 10
func noteHPAMaxReplicas(obj interface{}) int64 {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return 0
	}
	spec, _ := m["spec"].(map[string]interface{})
	if spec == nil {
		return 0
	}
	return toInt64(spec["maxReplicas"])
}

// noteHPAScaling returns true when the HPA has a valid metric target and
// currentReplicas differs from desiredReplicas — a real scale-out or scale-in
// is in progress. Returns false when the metric is unknown (ScalingActive=False),
// which would otherwise produce a misleading true due to desiredReplicas=0.
//
//	{{ hpaScaling .children.hpa }}
func noteHPAScaling(obj interface{}) bool {
	if !noteHPAScalingActive(obj) {
		return false
	}
	return noteHPACurrentReplicas(obj) != noteHPADesiredReplicas(obj)
}

// noteHPAAtMax returns true when currentReplicas has reached maxReplicas.
// Useful for surfacing capacity pressure in status.
//
//	{{ hpaAtMax .children.hpa }}
func noteHPAAtMax(obj interface{}) bool {
	max := noteHPAMaxReplicas(obj)
	if max == 0 {
		return false
	}
	return noteHPACurrentReplicas(obj) >= max
}

// ── HPA condition notes ───────────────────────────────────────────────────────

// noteHPAScalingActive returns true when the ScalingActive condition is True —
// meaning the HPA can read its metric and is allowed to scale.
// False when the metric source is unknown or the HPA is disabled.
//
//	{{ hpaScalingActive .children.hpa }}
func noteHPAScalingActive(obj interface{}) bool {
	return noteHasCondition(obj, "ScalingActive")
}

// noteHPAAbleToScale returns true when the AbleToScale condition is True —
// meaning the scale target exists and is reachable.
//
//	{{ hpaAbleToScale .children.hpa }}
func noteHPAAbleToScale(obj interface{}) bool {
	return noteHasCondition(obj, "AbleToScale")
}

// noteHPAScalingLimited returns true when the ScalingLimited condition is True —
// meaning the desired replica count was clamped by min or max bounds.
//
//	{{ hpaScalingLimited .children.hpa }}
func noteHPAScalingLimited(obj interface{}) bool {
	return noteHasCondition(obj, "ScalingLimited")
}

// ── Enriched HPA notes ────────────────────────────────────────────────────────

// noteHPAScaleTargetName reads _scaleTarget.name.
// Requires enrich: [hpa] on the CRD.
//
//	{{ hpaScaleTargetName .children.hpa }}  → "my-app"
func noteHPAScaleTargetName(obj interface{}) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	target, _ := m["_scaleTarget"].(map[string]interface{})
	if target == nil {
		return ""
	}
	v, _ := target["name"].(string)
	return v
}

// noteHPAScaleTargetKind reads _scaleTarget.kind.
// Requires enrich: [hpa] on the CRD.
//
//	{{ hpaScaleTargetKind .children.hpa }}  → "Deployment"
func noteHPAScaleTargetKind(obj interface{}) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	target, _ := m["_scaleTarget"].(map[string]interface{})
	if target == nil {
		return ""
	}
	v, _ := target["kind"].(string)
	return v
}

// noteHPAMetricTypes reads _currentMetrics and returns a comma-separated list of type values.
// Requires enrich: [hpa] on the CRD.
//
//	{{ hpaMetricTypes .children.hpa }}  → "Resource, External"
func noteHPAMetricTypes(obj interface{}) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	metrics, _ := m["_currentMetrics"].([]interface{})
	var types []string
	for _, entry := range metrics {
		em, _ := entry.(map[string]interface{})
		if em == nil {
			continue
		}
		if t, _ := em["type"].(string); t != "" {
			types = append(types, t)
		}
	}
	result := ""
	for i, t := range types {
		if i > 0 {
			result += ", "
		}
		result += t
	}
	return result
}
