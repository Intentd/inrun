# Getting started

Inrun lets you take part in Kubernetes reconciliation without building an operator. You decide what a resource should become, in a Blueprint, a service or Go, and Inrun runs the informers, queues, workers, retries, leader election, status, cleanup and metrics around that decision.

1. [Install](01-install.md) the `inrun` CLI.
2. Work through [Learning Inrun](02-learning-inrun.md): your first reconciled resource
   with `inrun init`, then six example packs, in order, each one checked
   without a cluster first and then end to end.

You need `kubectl` and a cluster you can reach for the end-to-end steps;
[kind](https://kind.sigs.k8s.io/) is enough. Everything before that runs on
your machine alone.
