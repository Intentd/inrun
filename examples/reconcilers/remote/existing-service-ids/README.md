# Existing service that assigns ids

A `Volume` operator whose reconciler is a storage API that assigns the id
itself. `POST /volumes` creates a volume and answers its id; every later call
addresses it as `/volumes/{id}`. Most real APIs work this way, and it is
harder to reconcile safely than an API keyed by a name you choose (see
[../existing-service](../existing-service/)).

The dev server plays the API: a new volume is `provisioning` until it is
read once, then `ready`. A `DELETE` answers `202` once, then `404` when the
volume is gone. `GET /volumes/requests` lists every request it received.

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
kubectl get volume data
curl -s localhost:9999/volumes/requests
```

## The pattern

Create once, then read by the id the service assigned:

```yaml
reconcile:
  external:
    - name: current                         # read by id, once there is one
      url: "{{ volumeAPI }}/volumes/{{ .status.volumeId }}"
      when:
        - field: status.volumeId
          exists: true
      expectedStatus: [200, 404]
  remote:
    endpoint: "{{ volumeAPI }}/volumes"
    request:
      method: POST                          # create only while there is no volume
      body: '{"name": "{{ .metadata.name }}", "sizeGB": {{ .args.sizeGB }}}'
      expectedStatus: [201]
      or:
        - field: status.volumeId
          notExists: true
        - field: external.current.status    # deleted outside the cluster
          equals: "404"
      finalize:
        url: "{{ volumeAPI }}/volumes/{{ .status.volumeId }}"
        method: DELETE
        expectedStatus: [202, 404]
    response:
      status:
        fields:
          - path: volumeId                  # written on the create pass only
            value: "{{ .response.json.id }}"
```

Pass by pass: the first POSTs and stores `volumeId`; later ones read the
volume by id and mirror its state through `emit.status`; a delete removes it
by id and releases the CR at `404`.

## Why the create is safe to retry

The id exists only once the status patch with it lands. If that patch fails,
or the next pass reads the CR from a cache that has not seen it yet,
"no `volumeId` yet" holds again and the POST runs a second time.

Inrun sends an `Idempotency-Key` header with every POST and PATCH, derived
from the CR's UID and the request. A repeated create sends the same key, and
an API that honours it (as this one and Stripe-style APIs do) answers the
first volume instead of creating another. Nothing to write in the Catalog.

When the API wants a different header or its own key format:

```yaml
request:
  idempotencyKey:
    header: PayPal-Request-Id
    value: "{{ .metadata.namespace }}-{{ .metadata.name }}"
```

`idempotencyKey: false` turns it off.

An API with neither idempotency keys nor a lookup by a name the CR controls
(`GET /volumes?name=...`) cannot be reconciled safely by anything: Inrun or a
hand-written operator.

## Cleanup

```bash
./cleanup.sh
```
