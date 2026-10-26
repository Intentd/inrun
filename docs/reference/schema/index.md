# Schema

Every field Inrun reads, by file kind.

| Section | Covers |
| --- | --- |
| [Resources](resources/index.md) | The Kubernetes kinds declarable under `onCreate`, `onReconcile` and `onDelete`, one page per kind |
| [E2E](e2e/complete-example.md) | A complete `e2e.yaml` |

The Blueprint's own blocks (`metadata`, `spec.crds`, `operatorBox`, `serve`,
`admission`, `migrate`, `security`, `gateway`, `presets`, `functions`, `lifecycle`,
`publish`) are
described in [The Blueprint](../../concepts/01-blueprint.md); `inrun validate`
checks them in that order. The functions templates can call are in
[Functions](../inrun-functions/index.md).
