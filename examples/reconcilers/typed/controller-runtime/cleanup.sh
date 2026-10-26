#!/usr/bin/env bash
kubectl delete -f manifests/cr.yaml --ignore-not-found
kubectl delete -f manifests/crd.yaml --ignore-not-found
