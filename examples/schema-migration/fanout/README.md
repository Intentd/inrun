# schema-evolution

A `CronJob` operator that walks through a real schema migration. The CRD starts
with a flat cron string in `spec.schedule`. The schema evolves to a structured
object with five fields. Existing CRs written against the old schema stay in
the cluster as-is until `inrun migrate` backfills them.

## The schema change

**Old** — `spec.schedule` was a string:

```yaml
spec:
  schedule: "0 2 * * 1-5"
  image: "postgres:16-alpine"
```

**New** — `spec.schedule` is a structured object:

```yaml
spec:
  schedule:
    minute:     "0"
    hour:       "2"
    dayOfMonth: "*"
    month:      "*"
    dayOfWeek:  "1-5"
  image: "postgres:16-alpine"
```

The serve layer keeps the caller interface flat — you still submitv`schedule: "0 2 * * 1-5"` and inrun fans it out. The `migrate.from: schedule`vdeclaration in [`serve/cronjob.yaml`](./serve/cronjob.yaml) is what tells the migrator where the oldvvalue lives.

---

## Walkthrough

### 1. Apply the old CRD and namespaces

```bash
kubectl apply -f manifests/old-crd.yaml
kubectl apply -f manifests/namespaces.yaml
```

### 2. Create CRs with the old flat-string schema

Three teams, three namespaces, each with a CronJob using the old schema.

```bash
kubectl apply -f manifests/old-crs.yaml
```

### 3. Observe the old schema

```bash
kubectl get cronjobs.demo.inrun.dev -A
```

Expected output — `SCHEDULE` shows the flat cron string:

```
NAMESPACE    NAME         SCHEDULE        IMAGE                  AGE
team-alpha   db-backup    0 2 * * 1-5     postgres:16-alpine     5s
team-beta    cache-flush  */15 * * * *    redis:7-alpine         5s
team-gamma   report-gen   0 8 * * 1       python:3.12-slim       5s
```

Inspect one CR directly:

```bash
kubectl get cronjobs.demo.inrun.dev db-backup -n team-alpha \
  -o jsonpath='{.spec}' | jq
```

```json
{"image":"postgres:16-alpine","schedule":"0 2 * * 1-5"}
```

### 4. Apply the new CRD

The schema evolves: `spec.schedule` is now a structured object. Kubernetes
replaces the CRD definition but does not touch existing CRs — they stay in etcd
with the old flat string, which no longer matches the new schema.

```bash
kubectl apply -f manifests/crd.yaml
```

The CRs are still there:

```bash
kubectl get cronjobs.demo.inrun.dev -A
```

But the old flat string is now unstructured data in an object-typed field:

```bash
kubectl get cronjobs.demo.inrun.dev db-backup -n team-alpha \
  -o jsonpath='{.spec}' | jq
```

```json
{"image":"postgres:16-alpine","schedule":"0 2 * * 1-5"}
```

### 5. Validate the catalog

```bash
inrun validate
```

### 6. Run the migration

`inrun migrate` reads the `migrate.from` declarations in the catalog, finds
every CR across all namespaces where the old path is present, applies the same
fanout transform the serve layer uses, and removes the old path.

Dry-run first if you want to preview without writing:

```bash
inrun migrate --dry-run
```

```bash
inrun migrate
```

Run it again to confirm idempotency — CRs already in the new form are skipped:

```
◆  CronJob  3 CRs
  –  team-alpha/db-backup    (already migrated)
  –  team-beta/cache-flush   (already migrated)
  –  team-gamma/report-gen   (already migrated)

  ✓ 0/3 migrated · 3 already done
```

### 7. Verify the migration

```bash
kubectl get cronjobs.demo.inrun.dev -A
```

```bash
kubectl get cronjobs.demo.inrun.dev db-backup -n team-alpha \
  -o jsonpath='{.spec.schedule}'  | jq
```

Expected:

```json
{"dayOfMonth":"*","dayOfWeek":"1-5","hour":"2","minute":"0","month":"*"}
```

All three CRs across all namespaces are now in the new structured form.

### 8. Clean up

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

---

## Running inrun with migration on startup

After cleanup, the migration step can be folded into startup. When `--migrate`
is set, inrun runs the migration before the gateway opens for traffic. If
migration fails, the gateway refuses to start.

```bash
kubectl apply -f manifests/old-crd.yaml
kubectl apply -f manifests/namespaces.yaml
kubectl apply -f manifests/old-crs.yaml

DEV_TOKEN=dev-token inrun --migrate
```

The gateway log will show the migration completing before it starts serving.

---

## Simulate

```bash
inrun simulate
```

## E2E

```bash
inrun e2e
```
