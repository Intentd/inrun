# pkg/external/fixtures

One fixture per supported protocol. Each verifies that the Inrun external call pipeline resolves, fetches, caches, and surfaces results correctly for that protocol.

| Fixture | Protocol | Backing service |
|---|---|---|
| [http-auth](http-auth/README.md) | `http` | Built-in `--dev-server` |
| [prometheus](prometheus/README.md) | `prometheus` | Inrun's own metrics server |
| [redis](redis/README.md) | `redis` | Redis — `docker-compose` |
| [postgres](postgres/README.md) | `postgres` | PostgreSQL — `docker-compose` |
| [mongo](mongo/README.md) | `mongo` | MongoDB — `docker-compose` |
| [kafka](kafka/README.md) | `kafka` | Kafka — `docker-compose` |

## Run the full e2e suite

```sh
inrun e2e pkg/external/fixtures/e2e.yaml
```