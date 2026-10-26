# health fixture

Living fixture for the Inrun health server. A minimal declarative operator
used only to boot the runtime; assertions target `/health`, `/ready`, and
`/startup` on the leader pod.

```bash
inrun e2e -f pkg/health/fixture/e2e.yaml
```
