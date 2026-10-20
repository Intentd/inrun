# Writing content

## Edit an existing page

Every page maps directly from `documentation/` to its URL:

| File | URL |
|---|---|
| `documentation/getting-started/index.md` | `/docs/getting-started/` |
| `documentation/inrun-registry/blueprints.md` | `/docs/inrun-registry/blueprints/` |
| `documentation/blog/why-inrun.md` | `/blog/why-inrun/` |

Open the file, edit, save. The sync handles the rest.

## Front matter

Do not add front matter manually. The sync script derives it from the first `# Heading` in the file and the file's git history. If your page starts with a `# Title`, the sync strips it and uses it as the Hugo `title` field — so do not repeat it in the body.

## Admonitions

Use the MkDocs `!!!` syntax — the sync converts it to the Hugo callout shortcode:

```markdown
!!! note "Optional title"
    Body text indented four spaces.

!!! warning
    No title — the type is used as the label.
```

Supported types: `note`, `warning`, `tip`, `danger`.

## Code blocks

Standard fenced code blocks. Always specify a language for syntax highlighting:

````markdown
```bash
inrun
```

```yaml
spec:
  crds:
    website:
      operatorBox:
        reconcile:
          workers: 3
```

```text
postgres/
  blueprint.yaml
  crd.yaml
```
````

Use `text` for directory trees or plain output. No language tag renders unstyled.

## Links between pages

Use relative `.md` paths when linking to other docs pages. The sync rewrites them to Hugo paths automatically:

```markdown
[Learning Inrun](../getting-started/learning-inrun.md)
[Blueprint schema](../reference/schema/02-blueprint/01-top-level.md)
```

Do not use absolute `/docs/...` paths inside documentation source files — they bypass the rewriter and break in local preview.

→ Next: [03-new-page](03-new-page.md)
