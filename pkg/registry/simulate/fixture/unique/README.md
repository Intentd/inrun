# unique

Demonstrates `operator: unique` — field value must be unique across all
existing instances of a CRD. Works the same way in `validation.rules` and in
`when:`/`or:` (see `status.fields.domainUnique` below), enforced only at
reconcile time via a live checker the reconciler injects into the resolver.

## Run

```sh
# Passes — no other Website shares this domain:
inrun simulate -f pkg/registry/simulate/fixture/unique/blueprint.yaml \
  --cr pkg/registry/simulate/fixture/unique/cr.yaml --cycles 2

# Denied — cr-duplicate.yaml's second document is a pre-existing Website
# with the same domain, seeded into the fake dynamic client so the checker
# actually finds it:
inrun simulate -f pkg/registry/simulate/fixture/unique/blueprint.yaml \
  --cr pkg/registry/simulate/fixture/unique/cr-duplicate.yaml --cycles 1
```

`cr-duplicate.yaml`'s two documents are both `kind: Website`. Only the
first is reconciled — the second is a pre-existing instance, not a peer CRD.
