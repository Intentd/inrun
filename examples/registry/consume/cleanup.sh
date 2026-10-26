#!/usr/bin/env bash
set -euo pipefail

echo "Cleaning up consume..."

# Run this while inrun is still running: it removes what the CR created.
kubectl delete application my-app -n default --ignore-not-found --timeout=60s
kubectl delete crd applications.apps.inrun.dev --ignore-not-found

echo "✓ Done. Stop 'inrun' with Ctrl+C if still running."
