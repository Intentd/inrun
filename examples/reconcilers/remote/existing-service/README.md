# Existing service

A `Bucket` operator whose reconciler is a storage API that knows nothing about
Inrun. It takes `PUT /buckets/{name}` with its own body and answers in its own
shape. Inrun calls it once per reconcile and maps the answer into the CR.

The dev server plays the API: the first `PUT` answers `provisioning`, later
ones `ready`. A `DELETE` answers `202` once, then `404` when the bucket is
gone.

### 1. Simulate

No cluster needed. `--dev-server` starts the API on :9999.

```bash
inrun simulate --dev-server
```

### 2. Run

```bash
inrun --dev-server
```

### 3. Check the result

```bash
kubectl get bucket logs
```

## The mapping

`remote.response` turns on the mapping. Without it Inrun POSTs its own
request and expects its own reply (see [../endpoint](../endpoint/)).

```yaml
remote:
  endpoint: "{{ bucketAPI }}/buckets/{{ .metadata.name }}"
  request:
    method: PUT
    body: '{"quotaGB": {{ .args.quotaGB }}}'
    expectedStatus: [200]
    finalize:
      method: DELETE
      expectedStatus: [202, 404]
  response:
    status:
      fields:
        - path: bucketId
          value: "{{ .response.json.id }}"
    requeue:
      after: 5s
      or:
        - field: response.json.status
          equals: provisioning
        - field: response.status
          equals: "202"
```

- `request.body` is a template with `.args` and `.external` in scope. It
  replaces Inrun's request; `method` defaults to POST.
- Each call says which status codes mean success, in its own
  `expectedStatus` (below 400 when omitted). Anything else fails the
  reconcile with the usual backoff. A 404 on the PUT is an error, but on the
  DELETE it means the bucket is already gone, so it counts as done.
- The answer is `.response`: `status`, `body`, and `json` when the body is
  JSON.
- `response.status` fields are written to the CR, and `response.requeue`
  brings the CR back while the bucket is provisioning.
- While the CR is being deleted, `request.finalize` is called instead. The
  requeue holds the CR while the API answers `202`; at `404` it is released.
  Without `finalize`, nothing is called and the CR is released.

Nothing is created in the cluster: the CR is the record of what was asked
and what the API answered. To have the answer turn into Kubernetes resources,
see [../existing-service-resources](../existing-service-resources/).

### 4. Clean up

Run it while `inrun` is still up, so the delete reaches the API.

```bash
chmod +x cleanup.sh && ./cleanup.sh
```

### 5. E2E

Deploys the dev server into a kind cluster and checks the mapped status and
the delete.

```bash
inrun e2e --dev-server
```
