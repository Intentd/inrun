#!/usr/bin/env bash
set -euo pipefail
echo "Cleaning up hooks..."
kubectl delete -f manifests/cr.yaml --ignore-not-found
kubectl delete -f manifests/crd.yaml --ignore-not-found
echo "✓ Done. Stop 'inrun' with Ctrl+C if still running locally."
