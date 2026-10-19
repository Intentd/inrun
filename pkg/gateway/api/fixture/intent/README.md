# Intent Files

Flat YAML and JSON intent files for `inrun serve play`. Each file matches one of the three
surfaces defined in the fixture blueprint and corresponds to a walkthrough scenario.

| File | Target | Surface | Expected result |
|---|---|---|---|
| `primary.yaml` | `apifixture` | Primary target | All stages pass |
| `internal.yaml` | `internal` | Alias — full permissions | All stages pass, `alias=internal` in payload |
| `preview.json` | `preview` | Alias — read-only | Fails at stage 2 (token denied on `create`) |

## Run

```bash
BLUEPRINT=pkg/gateway/api/fixture/blueprint.yaml

inrun serve play -f $BLUEPRINT --token console -i pkg/gateway/api/fixture/intent/primary.yaml
inrun serve play -f $BLUEPRINT --token console -i pkg/gateway/api/fixture/intent/internal.yaml
inrun serve play -f $BLUEPRINT --token console -i pkg/gateway/api/fixture/intent/preview.json
```

The `preview` run is expected to stop at the token check stage — `console`
holds only `get` and `list` on that alias. The denial is the correct result.

To simulate what `ci-pipeline` can do:

```bash
# Allowed on primary (ci-pipeline has get/list at CRD level)
inrun serve play -f $BLUEPRINT --token ci-pipeline --operation list -i pkg/gateway/api/fixture/intent/primary.yaml --target apifixture

# Denied — ci-pipeline is not listed in the preview or internal alias token maps
inrun serve play -f $BLUEPRINT --token ci-pipeline -i pkg/gateway/api/fixture/intent/preview.json --target preview
```
