## E2E Results: retry-e2e

| Property | Value |
|---|---|
| Spec | e2e.yaml |
| Cluster | inrun-e2e |
| Duration | 3s |

| Result | |
|---|---|
| Total | 6 |
| ✅ Passed | 5 |
| ❌ Failed | 0 |
| ⏭ Skipped | 1 |

> 🔁 **2 expectation(s) needed a retry** — 3 extra attempt(s) total

**Command:**

```bash
inrun e2e --report
```

### Passed

| Test | Attempts | Time |
|---|---|---|
| ✅ passes first attempt |  | 4ms |
| ✅ passes on second attempt | 2 of 3 | 1.011s |
| ✅ passes on third attempt | 3 of 3 | 2.017s |
| ✅ passes with expect env |  | 2ms |
| ✅ passes with spec env |  | 2ms |

### Skipped

| Test | Attempts | Time |
|---|---|---|
| ⏭ skipped by when |  | 0s |

**5 of 6 passed, 1 skipped (3s)**

