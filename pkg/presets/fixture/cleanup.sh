#!/usr/bin/env bash
set -euo pipefail

echo "Cleaning up preset-probe fixture..."

kubectl delete -f cr.yaml --ignore-not-found
kubectl delete -f crd.yaml --ignore-not-found

echo "Done. Stop 'inrun' with Ctrl+C if still running."
