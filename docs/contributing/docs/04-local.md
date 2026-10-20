# Local preview

## Requirements

- [Hugo extended](https://gohugo.io/installation/) — version matches `HUGO_VERSION` in `.github/workflows/pages-deploy.yml` (currently `0.160.0`)
- Git (for the sync script to read commit dates)

## Run the sync and serve

```bash
# From the repo root — sync docs then start Hugo
bash website/scripts/sync-docs.sh && hugo server --source ./website
```

Hugo starts a live-reload server:

```text
Web Server is available at http://localhost:1313/
```

Edit any file in `documentation/` and re-run the sync to see the change. Hugo picks up the updated file instantly — no restart needed.

## Sync only (no server)

To check that the sync runs without errors before pushing:

```bash
bash website/scripts/sync-docs.sh
```

Output shows every file synced and any section indices auto-created:

```text
Cleaning content/docs ...
Syncing docs: .../documentation → .../website/content/docs
  docs: getting-started/index.md
  docs: inrun-registry/blueprints.md
  ...
Done.
```

## What CI runs

The deploy workflow runs exactly the same two steps:

```bash
bash website/scripts/sync-docs.sh
hugo --source ./website --gc --minify --baseURL "https://inrun.dev/"
```

If it passes locally, it passes in CI. If `hugo server` shows broken links or missing shortcodes, fix them before pushing — CI will fail on the same errors.
