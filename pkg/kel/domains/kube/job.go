package note

import "text/template"

// jobNotes registers helpers for inspecting Job and CronJob status fields.
//
// Usage:
//
//	tmpl.Funcs(note.jobNotes())
//
// Template examples:
//
//	{{ jobSucceeded .children.job }}
//	{{ jobFailed .children.job }}
//	{{ jobActive .children.job }}
//	{{ jobFirstExitCode .children.job }}
//	{{ jobActivePodNames .children.job }}
//	{{ jobSucceededPodNames .children.job }}
//	{{ jobFailedPodNames .children.job }}
//	{{ cronJobActiveCount .children.cronjob }}
//	{{ cronJobLastScheduleTime .children.cronjob }}
//	{{ cronJobNextScheduleTime .children.cronjob }}
//	{{ cronJobLastSuccessTime .children.cronjob }}
//
// Pod note functions (jobFirstExitCode, jobActivePodNames, etc.) require
// enrich: [pods] on the CRD.
func jobNotes() template.FuncMap {
	return template.FuncMap{
		"jobSucceeded": noteJobSucceeded,
		"jobFailed":    noteJobFailed,
		"jobActive":    noteJobActive,
		// Enriched pod notes — require enrich: [pods] on the CRD.
		"jobFirstExitCode":     noteJobFirstExitCode,
		"jobActivePodNames":    noteJobActivePodNames,
		"jobSucceededPodNames": noteJobSucceededPodNames,
		"jobFailedPodNames":    noteJobFailedPodNames,
		// CronJob notes.
		"cronJobActiveCount":      noteCronJobActiveCount,
		"cronJobLastScheduleTime": noteCronJobLastScheduleTime,
		"cronJobNextScheduleTime": noteCronJobNextScheduleTime,
		"cronJobLastSuccessTime":  noteCronJobLastSuccessTime,
		// Enriched CronJob last-job notes — require enrich: [cronjob] on the CRD.
		"cronJobLastJobName":           noteCronJobLastJobName,
		"cronJobLastJobSucceeded":      noteCronJobLastJobSucceeded,
		"cronJobLastSuccessfulJobName": noteCronJobLastSuccessfulJobName,
	}
}

// ── Job / batch notes ─────────────────────────────────────────────────────────

// noteJobSucceeded returns true when at least one pod has succeeded.
//
//	{{ jobSucceeded .children.job }}
func noteJobSucceeded(obj interface{}) bool {
	status := noteStatus(obj)
	return toInt64(status["succeeded"]) > 0
}

// noteJobFailed returns true when at least one pod has failed.
// Use to gate dependent resources on clean job completion:
//
//	when:
//	  - field: "{{ jobFailed .children.job }}"
//	    equals: "false"
//	  - field: "{{ jobSucceeded .children.job }}"
//	    equals: "true"
func noteJobFailed(obj interface{}) bool {
	status := noteStatus(obj)
	return toInt64(status["failed"]) > 0
}

// noteJobActive returns true when at least one pod is currently running.
//
//	{{ jobActive .children.job }}
func noteJobActive(obj interface{}) bool {
	status := noteStatus(obj)
	return toInt64(status["active"]) > 0
}

// ── Enriched job pod notes ────────────────────────────────────────────────────

// noteJobFirstExitCode returns the exit code of the first terminated pod in _pods.
// Returns -1 when no pod has terminated yet.
// Requires enrich: [pods] on the CRD.
//
//	{{ jobFirstExitCode .children.job }}  → 0
func noteJobFirstExitCode(obj interface{}) int64 {
	for _, p := range getPods(obj) {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		code := toInt64(pm["exitCode"])
		if code >= 0 {
			return code
		}
	}
	return -1
}

// noteJobActivePodNames returns comma-separated names of pods that are not yet done.
// Requires enrich: [pods] on the CRD.
//
//	{{ jobActivePodNames .children.job }}  → "my-job-abc, my-job-def"
func noteJobActivePodNames(obj interface{}) string {
	return filterJobPodNames(obj, func(phase string) bool {
		return phase != "Succeeded" && phase != "Failed"
	})
}

// noteJobSucceededPodNames returns comma-separated names of pods that succeeded.
// Requires enrich: [pods] on the CRD.
//
//	{{ jobSucceededPodNames .children.job }}  → "my-job-abc"
func noteJobSucceededPodNames(obj interface{}) string {
	return filterJobPodNames(obj, func(phase string) bool {
		return phase == "Succeeded"
	})
}

// noteJobFailedPodNames returns comma-separated names of pods that failed.
// Requires enrich: [pods] on the CRD.
//
//	{{ jobFailedPodNames .children.job }}  → "my-job-xyz"
func noteJobFailedPodNames(obj interface{}) string {
	return filterJobPodNames(obj, func(phase string) bool {
		return phase == "Failed"
	})
}

func filterJobPodNames(obj interface{}, match func(string) bool) string {
	var names []string
	for _, p := range getPods(obj) {
		pm, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		phase, _ := pm["phase"].(string)
		if match(phase) {
			name, _ := pm["name"].(string)
			if name != "" {
				names = append(names, name)
			}
		}
	}
	// reuse strings.Join via the strings package — already imported via note.go helpers
	result := ""
	for i, n := range names {
		if i > 0 {
			result += ", "
		}
		result += n
	}
	return result
}

// ── CronJob notes ─────────────────────────────────────────────────────────────

// noteCronJobActiveCount returns the number of currently active Job runs.
//
//	{{ cronJobActiveCount .children.cronjob }}  → 1
func noteCronJobActiveCount(obj interface{}) int {
	status := noteStatus(obj)
	active, _ := status["active"].([]interface{})
	return len(active)
}

// noteCronJobLastScheduleTime returns the last time the CronJob was scheduled.
// Returns "" when not yet scheduled.
//
//	{{ cronJobLastScheduleTime .children.cronjob }}  → "2026-05-19T10:00:00Z"
func noteCronJobLastScheduleTime(obj interface{}) string {
	status := noteStatus(obj)
	v, _ := status["lastScheduleTime"].(string)
	return v
}

// noteCronJobNextScheduleTime returns the next time the CronJob is scheduled to run.
// Returns "" when the field is absent (not all Kubernetes versions populate it).
//
//	{{ cronJobNextScheduleTime .children.cronjob }}  → "2026-05-19T11:00:00Z"
func noteCronJobNextScheduleTime(obj interface{}) string {
	status := noteStatus(obj)
	v, _ := status["nextScheduleTime"].(string)
	return v
}

// noteCronJobLastSuccessTime returns the last time the CronJob completed successfully.
// Returns "" when it has never succeeded.
//
//	{{ cronJobLastSuccessTime .children.cronjob }}  → "2026-05-19T10:00:00Z"
func noteCronJobLastSuccessTime(obj interface{}) string {
	status := noteStatus(obj)
	v, _ := status["lastSuccessfulTime"].(string)
	return v
}

// ── Enriched CronJob last-job notes ──────────────────────────────────────────

// noteCronJobLastJobName reads _lastJob.metadata.name.
// Requires enrich: [cronjob] on the CRD.
//
//	{{ cronJobLastJobName .children.cronjob }}  → "my-job-28600000"
func noteCronJobLastJobName(obj interface{}) string {
	return cronJobLastJobMetaName(obj, "_lastJob")
}

// noteCronJobLastJobSucceeded returns true when _lastJob.status.succeeded > 0.
// Requires enrich: [cronjob] on the CRD.
//
//	{{ cronJobLastJobSucceeded .children.cronjob }}
func noteCronJobLastJobSucceeded(obj interface{}) bool {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return false
	}
	job, _ := m["_lastJob"].(map[string]interface{})
	if job == nil {
		return false
	}
	status, _ := job["status"].(map[string]interface{})
	return toInt64(status["succeeded"]) > 0
}

// noteCronJobLastSuccessfulJobName reads _lastSuccessfulJob.metadata.name.
// Requires enrich: [cronjob] on the CRD.
//
//	{{ cronJobLastSuccessfulJobName .children.cronjob }}  → "my-job-28599900"
func noteCronJobLastSuccessfulJobName(obj interface{}) string {
	return cronJobLastJobMetaName(obj, "_lastSuccessfulJob")
}

func cronJobLastJobMetaName(obj interface{}, key string) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	job, _ := m[key].(map[string]interface{})
	if job == nil {
		return ""
	}
	meta, _ := job["metadata"].(map[string]interface{})
	if meta == nil {
		return ""
	}
	v, _ := meta["name"].(string)
	return v
}
