# Job

Read Jobs and CronJobs: whether a Job succeeded, failed or is running, and when a CronJob last ran, last succeeded and runs next. The functions that look inside a Job's pods take the pods as a list.

## Reference

| Function | Description |
|----------|-------------|
| [`cronJobActiveCount`](#cronjobactivecount) | Returns the number of currently active Job runs (length of `status.active`). |
| [`cronJobLastScheduleTime`](#cronjoblastscheduletime) | Returns the last time the CronJob was scheduled (`status.lastScheduleTime`). |
| [`cronJobLastSuccessTime`](#cronjoblastsuccesstime) | Returns the last time the CronJob completed successfully (`status.lastSuccessfulTime`). |
| [`cronJobNextScheduleTime`](#cronjobnextscheduletime) | Returns when the CronJob runs next, computed from its spec the way the CronJob controller schedules it: spec.schedule in spec.timeZone (the controller's zone, UTC here, when unset). |
| [`jobActive`](#jobactive) | Returns `true` when `status.active > 0` — at least one pod is currently running. |
| [`jobActivePodNames`](#jobactivepodnames) | Returns the names of the job's pods that are not done (neither Succeeded nor Failed). |
| [`jobFailed`](#jobfailed) | Returns `true` when `status.failed > 0` — at least one pod has failed (and not been retried to success). |
| [`jobFailedPodNames`](#jobfailedpodnames) | Returns the names of the job's pods that failed. |
| [`jobFirstExitCode`](#jobfirstexitcode) | Returns the exit code of the first terminated container among the job's pods; -1 when none has terminated yet. |
| [`jobSucceeded`](#jobsucceeded) | Returns `true` when `status.succeeded > 0` — at least one pod has completed successfully. |
| [`jobSucceededPodNames`](#jobsucceededpodnames) | Returns the names of the job's pods that succeeded. |

## Functions

### cronJobActiveCount

`cronJobActiveCount` returns the number of currently active Job runs (length of `status.active`).

```text
{{ cronJobActiveCount .cronjob }}  → 1
```

### cronJobLastScheduleTime

`cronJobLastScheduleTime` returns the last time the CronJob was scheduled (`status.lastScheduleTime`). Returns `""` when not yet scheduled.

```text
{{ cronJobLastScheduleTime .cronjob }}  → "2026-05-19T10:00:00Z"
```

### cronJobLastSuccessTime

`cronJobLastSuccessTime` returns the last time the CronJob completed successfully (`status.lastSuccessfulTime`). Returns `""` when it has never succeeded.

```text
{{ cronJobLastSuccessTime .cronjob }}  → "2026-05-19T10:00:00Z"
```

### cronJobNextScheduleTime

`cronJobNextScheduleTime` returns when the CronJob runs next, computed from its spec the way the CronJob controller schedules it: spec.schedule in spec.timeZone (the controller's zone, UTC here, when unset). "" when the CronJob is suspended or has no schedule; an error when the schedule or time zone does not parse.

```text
{{ cronJobNextScheduleTime .cronjob }}  → "2026-05-19T11:00:00Z"
```

See also: [`nextCron`](../time/#nextcron), [`cronJobLastScheduleTime`](#cronjoblastscheduletime)

### jobActive

`jobActive` returns `true` when `status.active > 0` — at least one pod is currently running.

```text
{{ jobActive .job }}  → false
```

### jobActivePodNames

`jobActivePodNames` returns the names of the job's pods that are not done (neither Succeeded nor Failed).

```text
{{ jobActivePodNames .jobPods }}  → [migrate-r2d5m]
```

### jobFailed

`jobFailed` returns `true` when `status.failed > 0` — at least one pod has failed (and not been retried to success).

```text
{{ jobFailed .job }}  → false
```

### jobFailedPodNames

`jobFailedPodNames` returns the names of the job's pods that failed.

```text
{{ jobFailedPodNames .jobPods }}  → [migrate-x7k2p]
```

### jobFirstExitCode

`jobFirstExitCode` returns the exit code of the first terminated container among the job's pods; -1 when none has terminated yet.

```text
{{ jobFirstExitCode .jobPods }}  → 1
```

### jobSucceeded

`jobSucceeded` returns `true` when `status.succeeded > 0` — at least one pod has completed successfully.

```text
{{ jobSucceeded .job }}  → true
```

### jobSucceededPodNames

`jobSucceededPodNames` returns the names of the job's pods that succeeded.

```text
{{ jobSucceededPodNames .jobPods }}  → [migrate-q9w4z]
```
