#!/bin/bash
set -e

helm uninstall inrun --namespace inrun-system 2>/dev/null || true
kubectl delete -f bundle.yaml --ignore-not-found
kubectl delete -f crd.yaml --ignore-not-found
kubectl delete namespace team-payments --ignore-not-found
kubectl delete namespace argocd --ignore-not-found
helm uninstall cert-manager --namespace cert-manager 2>/dev/null || true
helm uninstall crossplane --namespace crossplane-system 2>/dev/null || true
