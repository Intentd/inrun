#!/usr/bin/env bash
set -euo pipefail

echo "Cleaning up publish..."

# Run this while inrun is still running: it removes what the CR created.
kubectl delete webservice orders -n default --ignore-not-found --timeout=60s
kubectl delete crd webservices.platform.example.com --ignore-not-found

echo "✓ Done. Stop 'inrun' with Ctrl+C if still running."
