#!/usr/bin/env bash
set -euo pipefail
kubectl delete cronjobs.demo.inrun.dev --all -n team-alpha --ignore-not-found
kubectl delete cronjobs.demo.inrun.dev --all -n team-beta  --ignore-not-found
kubectl delete cronjobs.demo.inrun.dev --all -n team-gamma --ignore-not-found
kubectl delete cronjobs.demo.inrun.dev --all -n default    --ignore-not-found
kubectl delete crd cronjobs.demo.inrun.dev --ignore-not-found
kubectl delete namespace team-alpha team-beta team-gamma --ignore-not-found
