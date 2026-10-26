# kafka

Verifies `protocol: kafka` using a Kafka instance queried from `spec.kafkaBrokers`.

One call is declared:

- `topicMeta` — `@events` topic metadata query. Returns partition count. `continueOnError: true` so the CR stays reconciling when the broker is temporarily unreachable.

## Local run (docker-compose)

```sh
docker compose -f pkg/external/fixtures/kafka/docker-compose.yaml up -d
inrun -f pkg/external/fixtures/kafka/blueprint.yaml
```

## Check the Status

```sh
kubectl get webapp my-app -oyaml
```

## E2e

Deploys a single-node Kafka instance in `inrun-system` before the CR is applied.

```sh
inrun e2e pkg/external/fixtures/kafka/e2e.yaml
```
