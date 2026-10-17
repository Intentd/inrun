# XX — Domain Title

One or two sentences introducing this domain of notes. Explain what these notes solve and why they exist.

## Reference

### `noteName`

A concise paragraph describing what the note does.
This must be the first non-empty paragraph after the heading — the generator uses it as the description.

Keywords: keyword1, keyword2, keyword3

```yaml
# Example usage of the note
# value: "{{ noteName .spec.field }}"
```

---

### `noteA` / `noteB`

Use this form when two notes share the same description and example.
The generator will register both names with the same metadata.

Keywords: keywordA, keywordB, shared, multi-note

```yaml
# Example demonstrating both noteA and noteB
# value: "{{ noteA .spec.field }}"
# value: "{{ noteB .spec.field }}"
```

---

## Quick reference

| Note | Accepts | Returns | Use in |
|------|---------|---------|--------|
| `noteName` | input types | output type | typical usage |
| `noteA` | input types | output type | typical usage |
| `noteB` | input types | output type | typical usage |

---

## Keyword guidance

`Keywords:` lines are parsed by the note list generator and power `inrun notes search`. They are **not** shown in the rendered docs — they exist purely for discovery.

**Rules:**
- Place `Keywords:` immediately after the note description, before the code block.
- Use lowercase, comma-separated terms. No phrases — single words only.
- 4–8 keywords per note is enough. More is noise.
- Always include the **domain** (e.g. `cron`, `hpa`, `pods`, `service`), the **return type** (`boolean`, `int`, `string`), and 2–4 **use-case** terms that describe what someone would search for.

**Good keywords** match what an operator author would type into search:
```
Keywords: hpa, autoscaler, replicas, scaling, boolean, active
Keywords: pods, crash, loop, enriched, health, boolean
Keywords: service, loadbalancer, ip, external, cloud, string
```

**Avoid** vague terms (`value`, `check`, `note`, `returns`) that apply to every note and add no signal.
