# http-auth

Verifies `auth.secretRef` on `external:` calls.

Two external calls are declared:

- `healthCheck` — unauthenticated `GET /health`.
- `config` — `GET /protected/:name` with `auth.secretRef` pointing to a Kubernetes Secret (`dev-server-token/default`, key `token`). The dev server validates `Authorization: Bearer dev-token-abc123` and returns 401 otherwise.

The fixture creates the Secret inline before the CR is applied, asserts `status.tokenValid == "true"`, then patches the Secret to a wrong token and asserts `status.configStatus` flips to `"401"` on the next reconcile.

This proves that `auth.secretRef` is resolved on every reconcile — a rotated Secret takes effect without restarting the operator.

## Local run

```sh
inrun -f pkg/external/fixtures/http-auth/blueprint.yaml --dev-server
```

`setup` applies `secret.yaml` (the bearer token) before the CR is created.

## Check the Status

```sh
kubectl get webapp my-app -oyaml
```

## E2e

```sh
inrun e2e pkg/external/fixtures/http-auth/e2e.yaml --dev-server
```
