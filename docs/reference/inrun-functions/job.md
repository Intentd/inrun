# Job Notes

Job notes read batch Job lifecycle state. They gate dependent resources on Job completion, failure, or active execution — the most common pattern in multi-step workflows where a migration or init job must finish before the main workload starts.

---

## Reference

| Note | Description |
|------|-------------|
| `jobSucceeded` | Return `true` when `status. |
| `jobFailed` | Return `true` when `status. |
| `jobActive` | Return `true` when `status. |
| `jobFirstExitCode` | Return the exit code of the first terminated pod in `_pods`. |
| `jobActivePodNames` | Return a comma-separated list of pod names that are not yet done (phase is not Succeeded or Failed). |
| `jobSucceededPodNames` | Return a comma-separated list of pod names that completed successfully. |
| `jobFailedPodNames` | Return a comma-separated list of pod names that failed. |
| `cronJobActiveCount` | Return the number of currently active Job runs (length of `status. |
| `cronJobLastScheduleTime` | Return the last time the CronJob was scheduled (`status. |
| `cronJobNextScheduleTime` | Return the next time the CronJob is scheduled to run (`status. |
| `cronJobLastSuccessTime` | Return the last time the CronJob completed successfully (`status. |
| `cronJobLastJobName` | Return the name of the most recently created Job (`_lastJob. |
| `cronJobLastJobSucceeded` | Return `true` when the most recently created Job has at least one succeeded pod. |
| `cronJobLastSuccessfulJobName` | Return the name of the most recently successful Job (`_lastSuccessfulJob. |

## Examples

```yaml
# jobSucceeded
# Gate the main deployment on a migration job:
when:
  - field: "{{ jobSucceeded .children.migrationjob }}"
    equals: "true"

# jobFailed
# Require both: not failed AND succeeded before proceeding:
when:
  - field: "{{ jobFailed .children.migrationjob }}"
    equals: "false"
  - field: "{{ jobSucceeded .children.migrationjob }}"
    equals: "true"

# jobActive
# Block cleanup until the job is no longer running:
when:
  - field: "{{ jobActive .children.cleanupjob }}"
    equals: "false"

# jobFirstExitCode
- path: exitCode
  value: "{{ jobFirstExitCode .children.migrationjob }}"
# → 0

# jobActivePodNames
- path: runningPods
  value: "{{ jobActivePodNames .children.migrationjob }}"
# → "my-job-abc, my-job-def"

# jobSucceededPodNames
- path: succeededPods
  value: "{{ jobSucceededPodNames .children.migrationjob }}"
# → "my-job-abc"

# jobFailedPodNames
- path: failedPods
  value: "{{ jobFailedPodNames .children.migrationjob }}"
# → "my-job-xyz"

# cronJobActiveCount
- path: activeRuns
  value: "{{ cronJobActiveCount .children.cronjob }}"
# → 1

# cronJobLastScheduleTime
- path: lastScheduled
  value: "{{ cronJobLastScheduleTime .children.cronjob }}"
# → "2026-05-19T10:00:00Z"

# cronJobNextScheduleTime
- path: nextScheduleTime
  value: "{{ cronJobNextScheduleTime .children.cronjob }}"
# → "2026-05-19T11:00:00Z"

# cronJobLastSuccessTime
- path: lastSuccess
  value: "{{ cronJobLastSuccessTime .children.cronjob }}"
# → "2026-05-19T10:00:00Z"

# cronJobLastJobName
- path: lastJobName
  value: "{{ cronJobLastJobName .children.cronjob }}"
# → "my-job-28600000"

# cronJobLastJobSucceeded
when:
  - field: "{{ cronJobLastJobSucceeded .children.cronjob }}"
    equals: "true"

# cronJobLastSuccessfulJobName
- path: lastSuccessfulJob
  value: "{{ cronJobLastSuccessfulJobName .children.cronjob }}"
# → "my-job-28599900"
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
