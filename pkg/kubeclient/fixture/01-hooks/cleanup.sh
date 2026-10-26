#!/usr/bin/env bash
set -euo pipefail
echo "Cleaning up blockchainapp hooks operator..."
kubectl delete blockchainapp 01-hooks-my-chain --ignore-not-found
kubectl delete -f crd.yaml --ignore-not-found
echo "✓ Done. Stop 'inrun' with Ctrl+C if still running locally."
