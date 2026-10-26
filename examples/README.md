# Examples

The intent loop, from the reconciler side and from the caller side. Each pack
is embedded in the CLI:

```bash
inrun init my-operator --pack declarative    # or remote, typed, intent, migration, registry
inrun init --list
```

| Pack | Where | Shows |
|------|-------|-------|
| `declarative` | [reconcilers/declarative](reconcilers/declarative/) | Operators declared in a Blueprint, no code. Start here. |
| `remote` | [reconcilers/remote](reconcilers/remote/) | A reconciler in any language, called over HTTP |
| `typed` | [reconcilers/typed](reconcilers/typed/) | Go hooks, a constructor, and a controller-runtime drop-in |
| `intent` | [intent](intent/) | Intent in through the gateway, view out |
| `migration` | [migration](migration/) | Change the CRD under live CRs: move fields, or move CRs to another CRD |
| `registry` | [registry](registry/) | Share operators as patterns: consume one, publish your own, deprecate it |

Every example has the same layout:

```text
blueprint.yaml      the operator; inrun, validate and generate read it
manifests/        what is applied to the cluster: the CRD, a CR, setup objects
test/             simulate.yaml, e2e.yaml and their values; found by inrun simulate and inrun e2e
cleanup.sh
```
