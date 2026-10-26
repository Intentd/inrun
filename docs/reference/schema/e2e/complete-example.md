# Complete E2E example

One file that exercises every `expect:` subcommand — `resources`, `commands`,
and every `kubectl:` subcommand. Use it as the canonical reference for what a
fully-featured E2E looks like.

Source: [`pkg/registry/e2e/fixture/e2e.yaml`](https://github.com/intentd/inrun/blob/main/pkg/registry/e2e/fixture/e2e.yaml)

---

```yaml
apiVersion: inrun.dev/v1
kind: E2E
metadata:
  name: e2e-probe
  description: >
    Living integration fixture for the kubectl DSL.
    Every subcommand supported by kubectl: must have at least one checkpoint here.
    When you add a new subcommand, add a checkpoint that exercises it.
    Two CRs run in parallel from cr.yaml:
      my-probe-server — inrun-dev-server on port 9999 (port-forward, JSON assertions)
      my-probe-exec   — nginx:alpine on port 80 (exec via sh)

spec:
  blueprint: ./blueprint.yaml
  crd: ./crd.yaml
  cr: ./cr.yaml

  cluster:
    provider: kind
    name: inrun-e2e-probe
    reuse: false

  expect:
    - name: The dev server is running (run with --dev-server)
      timeout: 30s
      onFailure:
        stop: true
      resources:
        - kind: Deployment
          name: inrun-dev-server
          namespace: inrun-system
          ready: true

    # ── resources: ──────────────────────────────────────────────────────────
    - name: All resources created and ready
      after: cr-applied
      timeout: 120s
      resources:
        - kind: E2EProbe
          name: my-probe-server
          namespace: default
        - kind: Deployment
          name: my-probe-server
          namespace: default
          ready: true
        - kind: Service
          name: my-probe-server-svc
          namespace: default
        - kind: E2EProbe
          name: my-probe-exec
          namespace: default
        - kind: Deployment
          name: my-probe-exec
          namespace: default
          ready: true
        - kind: Service
          name: my-probe-exec-svc
          namespace: default

    # ── kubectl.get ─────────────────────────────────────────────────────────
    - name: Both probes reach Ready status
      after: cr-applied
      timeout: 120s
      kubectl:
        get:
          - kind: E2EProbe
            name: my-probe-server
            namespace: default
            field: .status.phase
            assert:
              equals: Ready
          - kind: E2EProbe
            name: my-probe-exec
            namespace: default
            field: .status.phase
            assert:
              equals: Ready
      onFailure:
        kubectl:
          get:
            - kind: E2EProbe
              name: my-probe-server
              namespace: default
            - kind: E2EProbe
              name: my-probe-exec
              namespace: default
          describe:
            - kind: Deployment
              name: my-probe-server
              namespace: default
        commands:
          - run: kubectl get pods -n default -o wide

    - name: Field assertions on Deployments and Services
      after: cr-applied
      timeout: 60s
      kubectl:
        get:
          - kind: Deployment
            name: my-probe-server
            namespace: default
            field: .spec.replicas
            assert:
              equals: "1"
          - kind: Deployment
            name: my-probe-exec
            namespace: default
            field: .spec.replicas
            assert:
              equals: "1"
          - kind: Service
            name: my-probe-server-svc
            namespace: default
            field: .spec.type
            assert:
              equals: ClusterIP
          - kind: E2EProbe
            name: my-probe-server
            namespace: default
            field: .spec.message
            assert:
              equals: e2e-ready

    # ── kubectl.logs ─────────────────────────────────────────────────────────
    - name: Pod logs show expected startup output
      after: cr-applied
      timeout: 60s
      kubectl:
        logs:
          - labelSelector: inrun-owner=my-probe-server
            namespace: default
            assert:
              contains: "autoscale-metrics"
          - labelSelector: inrun-owner=my-probe-exec
            namespace: default
            assert:
              contains: "Configuration complete"
          - leaderElection:
              lease: inrun-leader
              namespace: inrun-system
            assert:
              contains: "became leader"

    # ── kubectl.describe ─────────────────────────────────────────────────────
    - name: Describe shows expected resource state
      after: cr-applied
      timeout: 30s
      kubectl:
        describe:
          - kind: Deployment
            name: my-probe-exec
            namespace: default
            assert:
              contains: RollingUpdate
          - kind: Service
            name: my-probe-server-svc
            namespace: default
            assert:
              contains: ClusterIP

    # ── kubectl.exec ─────────────────────────────────────────────────────────
    - name: Exec into nginx pod (has sh)
      after: cr-applied
      timeout: 60s
      kubectl:
        exec:
          - labelSelector: inrun-owner=my-probe-exec
            namespace: default
            command: [sh, -c, "echo e2e-ready"]
            assert:
              equals: e2e-ready

    # ── kubectl.port-forward ─────────────────────────────────────────────────
    - name: Port-forward to devserver health endpoints
      after: cr-applied
      timeout: 60s
      kubectl:
        port-forward:
          - service: my-probe-server-svc
            namespace: default
            port: 9999
            path: /health
            assert:
              contains: ok
          - service: my-probe-server-svc
            namespace: default
            port: 9999
            path: /started
            assert:
              contains: started
          - service: my-probe-server-svc
            namespace: default
            port: 9999
            path: /ready
            assert:
              contains: ready

    # ── kubectl.apply ────────────────────────────────────────────────────────
    - name: Apply a ConfigMap inline and assert it landed
      after: cr-applied
      timeout: 30s
      kubectl:
        apply:
          - inline: |
              apiVersion: v1
              kind: ConfigMap
              metadata:
                name: e2e-probe-extra
                namespace: default
              data:
                applied-by: kubectl-dsl
        get:
          - kind: ConfigMap
            name: e2e-probe-extra
            namespace: default
            field: .data.applied-by
            assert:
              equals: kubectl-dsl

    # ── kubectl.patch ────────────────────────────────────────────────────────
    - name: Patch server probe and assert the CR field updated
      after: cr-applied
      timeout: 30s
      kubectl:
        patch:
          - kind: E2EProbe
            name: my-probe-server
            namespace: default
            patch: '{"spec":{"message":"patched"}}'
        get:
          - kind: E2EProbe
            name: my-probe-server
            namespace: default
            field: .spec.message
            assert:
              equals: patched

    # ── kubectl.delete ───────────────────────────────────────────────────────
    - name: Delete the inline ConfigMap and verify it is gone
      after: cr-applied
      timeout: 30s
      kubectl:
        delete:
          - kind: ConfigMap
            name: e2e-probe-extra
            namespace: default
            ignoreNotFound: true
      resources:
        - kind: ConfigMap
          name: e2e-probe-extra
          namespace: default
          count: 0

    # ── kubectl.events ───────────────────────────────────────────────────────
    - name: Kubernetes events recorded for both Deployments
      after: cr-applied
      timeout: 60s
      kubectl:
        events:
          - kind: Deployment
            name: my-probe-server
            namespace: default
            assert:
              contains: ScalingReplicaSet
          - kind: Deployment
            name: my-probe-exec
            namespace: default
            assert:
              contains: ScalingReplicaSet

    # ── kubectl.auth ─────────────────────────────────────────────────────────
    - name: Current user can query cluster resources
      after: cr-applied
      timeout: 10s
      kubectl:
        auth:
          - verb: get
            resource: pods
            namespace: default
            assert:
              equals: "yes"
          - verb: list
            resource: deployments
            namespace: default
            assert:
              equals: "yes"

    # ── kubectl.cp ───────────────────────────────────────────────────────────
    - name: Copy file out of nginx container and assert content
      after: cr-applied
      timeout: 30s
      kubectl:
        cp:
          - labelSelector: inrun-owner=my-probe-exec
            namespace: default
            src: /etc/nginx/nginx.conf
            assert:
              contains: worker_processes

    # ── kubectl.top ──────────────────────────────────────────────────────────
    - name: Live resource usage visible via kubectl top
      after: cr-applied
      timeout: 60s
      kubectl:
        top:
          - kind: pod
            namespace: default
            labelSelector: inrun-owner=my-probe-server
            assert:
              contains: my-probe-server
          - kind: pod
            namespace: default
            labelSelector: inrun-owner=my-probe-exec
            assert:
              contains: my-probe-exec

    # ── commands: (arbitrary shell) ──────────────────────────────────────────
    - name: Arbitrary commands still work alongside the DSL
      after: cr-applied
      timeout: 30s
      commands:
        - run: kubectl get e2eprobe -n default -o name
          assert:
            contains: my-probe-server

    # ── kubectl.restart ──────────────────────────────────────────────────────
    - name: Rollout restart recovers the probe deployment
      after: cr-applied
      timeout: 120s
      kubectl:
        restart:
          - kind: Deployment
            name: my-probe-server
            namespace: default
      resources:
        - kind: Deployment
          name: my-probe-server
          namespace: default
          ready: true

    # ── kubectl.scale ────────────────────────────────────────────────────────
    # Use a standalone deployment — scaling a reconciled deployment is immediately
    # undone by the Inrun reconciler restoring the CR's replica count.
    - name: Scale standalone deployment up to 2 replicas
      after: cr-applied
      timeout: 60s
      kubectl:
        apply:
          - inline: |
              apiVersion: apps/v1
              kind: Deployment
              metadata:
                name: e2e-scale-target
                namespace: default
              spec:
                replicas: 1
                selector:
                  matchLabels:
                    app: e2e-scale-target
                template:
                  metadata:
                    labels:
                      app: e2e-scale-target
                  spec:
                    containers:
                      - name: nginx
                        image: nginx:alpine
        scale:
          - kind: Deployment
            name: e2e-scale-target
            namespace: default
            replicas: 2
        get:
          - kind: Deployment
            name: e2e-scale-target
            namespace: default
            field: .spec.replicas
            assert:
              equals: "2"

    - name: Scale standalone deployment back to 1 and clean up
      after: cr-applied
      timeout: 60s
      kubectl:
        scale:
          - kind: Deployment
            name: e2e-scale-target
            namespace: default
            replicas: 1
        delete:
          - kind: Deployment
            name: e2e-scale-target
            namespace: default
            ignoreNotFound: true

    # ── cleanup ──────────────────────────────────────────────────────────────
    - name: Cleanup verified
      after: cr-deleted
      timeout: 60s
      resources:
        - kind: E2EProbe
          name: my-probe-server
          namespace: default
          count: 0
        - kind: E2EProbe
          name: my-probe-exec
          namespace: default
          count: 0
        - kind: Deployment
          name: my-probe-server
          namespace: default
          count: 0
        - kind: Deployment
          name: my-probe-exec
          namespace: default
          count: 0

  # ── onFailure ───────────────────────────────────────────────────────────────
  # Runs once for the suite when any expectation fails. It is a post block:
  # the same commands, kubectl and external as a step, each run once with its
  # output printed. A failed operation or a missed assertion is a warning in
  # the report; it never changes the result.
  onFailure:
    kubectl:
      # kubectl get
      get:
        - kind: E2EProbe
          name: my-probe-server
          namespace: default
        - kind: E2EProbe
          name: my-probe-exec
          namespace: default
      # kubectl logs
      logs:
        - labelSelector: inrun-owner=my-probe-server
          namespace: default
          since: 2m
        - labelSelector: inrun-owner=my-probe-exec
          namespace: default
          since: 2m
      # kubectl describe
      describe:
        - kind: Deployment
          name: my-probe-server
          namespace: default
        - kind: Deployment
          name: my-probe-exec
          namespace: default
      # kubectl events
      events:
        - kind: Deployment
          name: my-probe-server
          namespace: default
        - kind: Deployment
          name: my-probe-exec
          namespace: default
      # kubectl exec
      exec:
        - labelSelector: inrun-owner=my-probe-exec
          namespace: default
          command: [sh, -c, "echo on-failure-exec-ok"]
    commands:
      - run: kubectl get pods -n default -o wide

  # ── finally ─────────────────────────────────────────────────────────────────
  # Runs once for the suite at its end, whether it passed or failed, before
  # teardown. A post block like onFailure: its failures are warnings. Use it to
  # undo what the suite changed outside the cluster.
  finally:
    commands:
      - run: echo "suite finished"
```

---

## What each checkpoint covers

| Checkpoint | Subcommand(s) |
|---|---|
| All resources created and ready | `resources:` |
| Both probes reach Ready status | `kubectl.get` — jsonpath field extraction |
| Field assertions on Deployments and Services | `kubectl.get` — multiple entries per block |
| Pod logs show expected startup output | `kubectl.logs` — `labelSelector`, `contains` |
| Describe shows expected resource state | `kubectl.describe` — kind + name |
| Exec into nginx pod (has sh) | `kubectl.exec` — `labelSelector`, inline command |
| Port-forward to devserver health endpoints | `kubectl.port-forward` — multiple paths per block |
| Apply a ConfigMap inline and assert it landed | `kubectl.apply` — inline manifest + follow-up `kubectl.get` |
| Patch server probe and assert the CR field updated | `kubectl.patch` — merge patch on spec field |
| Arbitrary commands still work alongside the DSL | `commands:` — raw shell alongside `kubectl:` |
| Cleanup verified | `resources:` with `count: 0` |

Two CRs run in parallel from `cr.yaml` (multi-document):

| CR | Image | Port | Purpose |
|---|---|---|---|
| `my-probe-server` | `ghcr.io/intentd/inrun-dev-server:latest` | 9999 | Port-forward and JSON endpoint assertions |
| `my-probe-exec` | `nginx:alpine` | 80 | Exec assertions — nginx has `sh`, the devserver is distroless |

See [`pkg/registry/e2e/fixture/README.md`](https://github.com/intentd/inrun/blob/main/pkg/registry/e2e/fixture/README.md) for
instructions on running this fixture and the rule for adding new subcommands.

---

→ Back: [Testing](../../../concepts/06-testing.md) | [Schema index](../index.md)
