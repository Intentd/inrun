# coordinator fixture

Living fixture for `pkg/runtime/coordinator`. Exercises the dependency health
state machine across three CRDs applied in phases:

- **Worker** — no dependencies
- **Component** — `dependsOn: Worker, condition: started`
- **Platform** — `dependsOn: Worker, condition: healthy`

A seed CRD/CR boots the runtime. Real CRDs and CRs are applied via
`kubectl.apply` steps in the e2e to observe each transition:

1. All three pending — CRDs declared but not yet installed
2. CRDs applied → all three pending (reconcilers start; retry loop fires every 90s)
3. Component CR applied → Component healthy; Worker and Platform pending
4. Platform CR applied → no reconcile fires (`totalReconciles=0`; Worker not yet healthy)
5. Worker CR applied → Worker healthy → Platform gate opens → Platform healthy

```bash
inrun e2e -f pkg/runtime/coordinator/fixture/e2e.yaml
```
