#!/usr/bin/env bash
set -e
kubectl delete -f cr.yaml --ignore-not-found
kubectl delete -f crd.yaml --ignore-not-found
kubectl delete configmap e2e-probe-extra -n default --ignore-not-found
