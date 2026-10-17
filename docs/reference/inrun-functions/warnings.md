# Warning Event Notes

Warning event notes navigate the `_warnings` enrichment embedded by the enrichment layer into Deployment, StatefulSet, Job, Service, and any other resource type. They let templates surface Kubernetes Warning events without hooks or custom Go code. All require `enrich: [events]` (or `enrichAll: true`) on the CRD.

---

## Reference

| Note | Description |
|------|-------------|
| `hasWarnings` | Returns `true` when `_warnings` contains at least one event. |
| `warningCount` | Returns the number of warning events as `int`. |
| `firstWarning` | Returns the message of the first warning event, or `""` when there are no warnings. |
| `warningMessages` | Returns a comma-separated list of all warning messages. |
| `warningReasons` | Returns a comma-separated list of all warning reasons (de-duplicated). |

## Examples

```yaml
# hasWarnings
when:
  - field: "{{ hasWarnings .children.deployment }}"
    equals: "false"

# warningCount
- path: warningCount
  value: "{{ warningCount .children.deployment }}"
# → 3

# firstWarning
- path: lastWarning
  value: "{{ firstWarning .children.deployment }}"
# → "Back-off restarting failed container"

# warningMessages
- path: warningMessages
  value: "{{ warningMessages .children.deployment }}"
# → "OOMKilled, CrashLoopBackOff"

# warningReasons
- path: warningReasons
  value: "{{ warningReasons .children.deployment }}"
# → "OOMKilled, BackOff"
```
