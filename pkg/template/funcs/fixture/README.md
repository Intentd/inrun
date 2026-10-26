# pkg/template/funcs/fixture

Integration fixture for KEL and Inrun's functions against a real cluster.

Unit tests feed functions hand-built maps; a live API returns objects with
extra fields, JSON-decoded numbers and absent optionals. Each Blueprint here
writes function results into the probe CR's status, so you can read them on the
object.

Every Blueprint uses the same probe CRD (`crd.yaml`) and CR (`cr.yaml`), and
covers one resource family: `blueprint.yaml` the general functions,
`blueprint-<family>.yaml` the rest (pods, statefulset, service, job, cronjob,
replicaset, pvc, ingress, hpa, warnings, when, pod-node).

## Run

```bash
cd pkg/template/funcs/fixture
inrun -f blueprint-pods.yaml          # or any other blueprint here
kubectl get noteprobe my-probe -o yaml -w
./cleanup.sh
```

`simulate/` holds a spec per Blueprint. CI's `simulate-fixtures` job runs each
with fake clients and under envtest, so the functions also read objects from a
real API server:

```bash
inrun simulate -f simulate/pods.yaml --envtest
```

## Adding a function

Add a `status.fields` entry to the Blueprint for its resource family:

```yaml
- path: myFunction
  value: "{{ myFunction .children.deployment }}"
```
