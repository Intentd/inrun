#!/usr/bin/env bash
set -euo pipefail

echo "Cleaning up intent..."

# The Websites came from the gateway, not a file. Delete them while inrun is
# still running, so it can remove their finalizers.
kubectl delete website --all --ignore-not-found
kubectl delete -f manifests/crd.yaml --ignore-not-found
kubectl delete -f manifests/secret.yaml --ignore-not-found
kubectl delete -f reconciler/deploy.yaml --ignore-not-found

echo "✓ Done. Stop 'inrun' and the reconciler with Ctrl+C if still running."
