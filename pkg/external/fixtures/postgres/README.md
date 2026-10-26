# postgres

Verifies `protocol: postgres` using a PostgreSQL instance queried from `spec.postgresURL`.

Two calls are declared:

- `dbName` — `SELECT current_database()`. Asserts connectivity and confirms the target database. `result` is `"inrun"` on a correctly configured instance.
- `connCount` — `SELECT COUNT(*) FROM pg_stat_activity`. Active connection count. Has `cacheFor: 10s`.

## Local run (docker-compose)

```sh
docker compose -f pkg/external/fixtures/postgres/docker-compose.yaml up -d
inrun -f pkg/external/fixtures/postgres/blueprint.yaml
```

## Check the Status

```sh
kubectl get webapp my-app -oyaml
```

## E2e

Deploys a PostgreSQL instance in `inrun-system` before the CR is applied.

```sh
inrun e2e pkg/external/fixtures/postgres/e2e.yaml
```
