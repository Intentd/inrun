# Tests

```bash
make test-unit          # no cluster
make test-race          # unit tests with the race detector
make test-integration   # envtest
make test-all           # unit + console + integration
make test-coverage      # writes coverage.html
```

| Tier | Where | Run with |
|------|-------|----------|
| Unit | `pkg/**/*_test.go` | `make test-unit` |
| Integration | `integration/` | `make test-integration` |
| Integration, declarative | `simulate-envtest/` | `inrun simulate --envtest` |
| Simulate | next to each Blueprint | `inrun simulate` |
| E2E | next to each Blueprint | `inrun e2e` |

`simulate-envtest/` runs the same checks as `integration/`, written as `simulate.yaml`.
