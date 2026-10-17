# 13 — Job Notes

Job notes read batch Job lifecycle state. They gate dependent resources on Job completion, failure, or active execution — the most common pattern in multi-step workflows where a migration or init job must finish before the main workload starts.

---

## Reference

### `jobSucceeded`

Return `true` when `status.succeeded > 0` — at least one pod has completed successfully.

Keywords: job, batch, succeeded, complete, boolean, migration, init

```yaml
# Gate the main deployment on a migration job:
when:
  - field: "{{ jobSucceeded .children.migrationjob }}"
    equals: "true"
```

---

### `jobFailed`

Return `true` when `status.failed > 0` — at least one pod has failed (and not been retried to success).

Keywords: job, batch, failed, error, boolean, retry, fault

```yaml
# Require both: not failed AND succeeded before proceeding:
when:
  - field: "{{ jobFailed .children.migrationjob }}"
    equals: "false"
  - field: "{{ jobSucceeded .children.migrationjob }}"
    equals: "true"
```

---

### `jobActive`

Return `true` when `status.active > 0` — at least one pod is currently running.

Keywords: job, batch, active, running, boolean, in-progress

```yaml
# Block cleanup until the job is no longer running:
when:
  - field: "{{ jobActive .children.cleanupjob }}"
    equals: "false"
```

---

### `jobFirstExitCode`

Return the exit code of the first terminated pod in `_pods`. Returns `-1` when no pod has terminated yet.

Requires `enrich: [pods]` on the CRD.

Keywords: job, batch, exit, code, pod, enriched, terminated

```yaml
- path: exitCode
  value: "{{ jobFirstExitCode .children.migrationjob }}"
# → 0
```

---

### `jobActivePodNames`

Return a comma-separated list of pod names that are not yet done (phase is not Succeeded or Failed).

Requires `enrich: [pods]` on the CRD.

Keywords: job, batch, active, pods, names, enriched, running

```yaml
- path: runningPods
  value: "{{ jobActivePodNames .children.migrationjob }}"
# → "my-job-abc, my-job-def"
```

---

### `jobSucceededPodNames`

Return a comma-separated list of pod names that completed successfully.

Requires `enrich: [pods]` on the CRD.

Keywords: job, batch, succeeded, pods, names, enriched, complete

```yaml
- path: succeededPods
  value: "{{ jobSucceededPodNames .children.migrationjob }}"
# → "my-job-abc"
```

---

### `jobFailedPodNames`

Return a comma-separated list of pod names that failed.

Requires `enrich: [pods]` on the CRD.

Keywords: job, batch, failed, pods, names, enriched, error

```yaml
- path: failedPods
  value: "{{ jobFailedPodNames .children.migrationjob }}"
# → "my-job-xyz"
```

---

## CronJob notes

### `cronJobActiveCount`

Return the number of currently active Job runs (length of `status.active`).

Keywords: cronjob, cron, active, count, int, scheduled

```yaml
- path: activeRuns
  value: "{{ cronJobActiveCount .children.cronjob }}"
# → 1
```

---

### `cronJobLastScheduleTime`

Return the last time the CronJob was scheduled (`status.lastScheduleTime`). Returns `""` when not yet scheduled.

Keywords: cronjob, cron, schedule, time, last, timestamp

```yaml
- path: lastScheduled
  value: "{{ cronJobLastScheduleTime .children.cronjob }}"
# → "2026-05-19T10:00:00Z"
```

---

### `cronJobNextScheduleTime`

Return the next time the CronJob is scheduled to run (`status.nextScheduleTime`). Returns `""` when the field is absent (not all Kubernetes versions populate it).

Keywords: cronjob, cron, schedule, time, next, timestamp

```yaml
- path: nextScheduleTime
  value: "{{ cronJobNextScheduleTime .children.cronjob }}"
# → "2026-05-19T11:00:00Z"
```

---

### `cronJobLastSuccessTime`

Return the last time the CronJob completed successfully (`status.lastSuccessfulTime`). Returns `""` when it has never succeeded.

Keywords: cronjob, cron, success, time, last, timestamp

```yaml
- path: lastSuccess
  value: "{{ cronJobLastSuccessTime .children.cronjob }}"
# → "2026-05-19T10:00:00Z"
```

---

### `cronJobLastJobName`

Return the name of the most recently created Job (`_lastJob.metadata.name`).

Requires `enrich: [cronjob]` on the CRD.

Keywords: cronjob, cron, job, name, enriched, last, recent

```yaml
- path: lastJobName
  value: "{{ cronJobLastJobName .children.cronjob }}"
# → "my-job-28600000"
```

---

### `cronJobLastJobSucceeded`

Return `true` when the most recently created Job has at least one succeeded pod.

Requires `enrich: [cronjob]` on the CRD.

Keywords: cronjob, cron, job, succeeded, boolean, enriched, last

```yaml
when:
  - field: "{{ cronJobLastJobSucceeded .children.cronjob }}"
    equals: "true"
```

---

### `cronJobLastSuccessfulJobName`

Return the name of the most recently successful Job (`_lastSuccessfulJob.metadata.name`). Different from `cronJobLastJobName` when the latest run is still in progress or has failed.

Requires `enrich: [cronjob]` on the CRD.

Keywords: cronjob, cron, job, name, successful, enriched, last

```yaml
- path: lastSuccessfulJob
  value: "{{ cronJobLastSuccessfulJobName .children.cronjob }}"
# → "my-job-28599900"
```

---

## Complete workflow pattern

```yaml
status:
  fields:
    - path: migrationSucceeded
      value: "{{ jobSucceeded .children.migrationjob }}"
    - path: migrationFailed
      value: "{{ jobFailed .children.migrationjob }}"
    - path: migrationRunning
      value: "{{ jobActive .children.migrationjob }}"

resources:
  - kind: Deployment
    name: app
    when:
      - field: status.migrationSucceeded
        equals: "true"
      - field: status.migrationFailed
        equals: "false"
```

---

## Quick reference

| Note | Signature | Returns | Requires |
|------|-----------|---------|----------|
| `jobSucceeded` | `(obj any)` | `bool` | — |
| `jobFailed` | `(obj any)` | `bool` | — |
| `jobActive` | `(obj any)` | `bool` | — |
| `jobFirstExitCode` | `(obj any)` | `int64` | `enrich: [pods]` |
| `jobActivePodNames` | `(obj any)` | `string` | `enrich: [pods]` |
| `jobSucceededPodNames` | `(obj any)` | `string` | `enrich: [pods]` |
| `jobFailedPodNames` | `(obj any)` | `string` | `enrich: [pods]` |
| `cronJobActiveCount` | `(obj any)` | `int` | — |
| `cronJobLastScheduleTime` | `(obj any)` | `string` | — |
| `cronJobNextScheduleTime` | `(obj any)` | `string` | — |
| `cronJobLastSuccessTime` | `(obj any)` | `string` | — |
| `cronJobLastJobName` | `(obj any)` | `string` | `enrich: [cronjob]` |
| `cronJobLastJobSucceeded` | `(obj any)` | `bool` | `enrich: [cronjob]` |
| `cronJobLastSuccessfulJobName` | `(obj any)` | `string` | `enrich: [cronjob]` |

---

**Next →** [14 — Service Notes](14-service.md)
