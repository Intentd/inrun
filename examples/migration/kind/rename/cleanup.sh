#!/usr/bin/env bash
set -euo pipefail

NAMESPACES="platform-01 platform-02 platform-03 platform-04 platform-05
            platform-06 platform-07 platform-08 platform-09 platform-10"

for ns in $NAMESPACES; do
  kubectl delete appconfig api-server -n "$ns" --ignore-not-found
done
kubectl delete newconfigs --all -A --ignore-not-found 2>/dev/null || true

kubectl delete crd appconfigs.demo.inrun.dev newconfigs.demo.inrun.dev --ignore-not-found

for ns in $NAMESPACES; do
  kubectl delete namespace "$ns" --ignore-not-found
done
