#!/usr/bin/env bash
set -euo pipefail

kubectl delete crd appconfigs.demo.inrun.dev appconfigs.platform.demo.inrun.dev --ignore-not-found
for ns in team-a team-b team-c team-d; do
  kubectl delete namespace "$ns" --ignore-not-found
done
