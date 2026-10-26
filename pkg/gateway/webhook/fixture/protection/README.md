# protection (webhook)

The three security-enforcement webhooks in one suite, run against a single
cluster: `admission` (validation + mutation), `deletion-protection`, and
`namespace-protection`. Adapted from `examples/security/` — self-contained
here so it can run as part of this package's own regression coverage rather
than depending on the example pack.

## Run

```sh
inrun e2e pkg/gateway/webhook/fixture/protection/e2e.yaml
```

Or each webhook individually — see the README in each subdirectory.
