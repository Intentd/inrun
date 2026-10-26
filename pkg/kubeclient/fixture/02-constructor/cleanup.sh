#!/usr/bin/env bash
set -euo pipefail
echo "Cleaning up blockchainnode constructor operator..."
kubectl delete blockchainnode 02-constructor-my-node --ignore-not-found
kubectl delete -f crd-node.yaml --ignore-not-found
echo "✓ Done. Stop 'inrun' with Ctrl+C if still running locally."
