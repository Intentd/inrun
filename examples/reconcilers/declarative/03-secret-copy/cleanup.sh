#!/usr/bin/env bash
set -euo pipefail

echo "Cleaning up 03-secret-copy..."

kubectl delete -f manifests/cr.yaml --ignore-not-found
kubectl delete -f manifests/crd.yaml --ignore-not-found
kubectl delete -f manifests/setup.yaml --ignore-not-found

echo "✓ Done. Stop 'inrun' with Ctrl+C if still running."
