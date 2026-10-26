# mongo

Verifies `protocol: mongo` using a MongoDB instance queried from `spec.mongoURL`.

Two calls are declared:

- `docs` — counts all documents in the `inrun.test` collection. Asserts connectivity. `continueOnError: true` so the CR stays reconciling when the instance is temporarily unreachable.
- `activeConnections` — counts documents matching `{"status": "active"}`. Has `cacheFor: 10s`.

## Local run (docker-compose)

```sh
docker compose -f pkg/external/fixtures/mongo/docker-compose.yaml up -d
inrun -f pkg/external/fixtures/mongo/blueprint.yaml
```

## Check the Status

```sh
kubectl get webapp my-app -oyaml
```

## E2e

Deploys a MongoDB instance in `inrun-system` before the CR is applied.

```sh
inrun e2e pkg/external/fixtures/mongo/e2e.yaml
```
