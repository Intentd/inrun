#!/usr/bin/env bash
set -euo pipefail

echo "Cleaning up existing-service..."

# Delete the CR while inrun is still running: the finalizing pass deletes the
# bucket and releases the CR once the API answers 404.
kubectl delete -f manifests/cr.yaml --ignore-not-found
kubectl delete -f manifests/crd.yaml --ignore-not-found

echo "✓ Done. Stop 'inrun' with Ctrl+C if still running."
