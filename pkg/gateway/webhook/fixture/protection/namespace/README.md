# namespace-protection (webhook)

Verifies `security.namespaceProtection`: a `ValidatingWebhookConfiguration`
rejects `CREATE`/`UPDATE` requests whose namespace violates the CRD's
`allowedNamespaces` (whitelist, `App`/`Database` → `production` only) or
`restrictedNamespaces` (blacklist, `Cache` → anywhere but `kube-system`/
`kube-public`) rule, before the CR is stored.

`cr-allowed.yaml` targets namespaces that pass both rules and reconciles
normally. `cr-blocked.yaml` targets namespaces each rule forbids and is
denied synchronously at `kubectl apply` time.

## Run

```sh
inrun e2e pkg/gateway/webhook/fixture/protection/namespace/e2e.yaml
```
