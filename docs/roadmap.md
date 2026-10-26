# Roadmap

What we plan after v0.1.0. These are directions, not dates; each one starts
as a GitHub discussion, and feedback there shapes the order.

## Simulate against both backends at once

`inrun simulate --envtest` runs the same `simulate.yaml` against a real API server instead of the in-memory fake, which is how the fake is kept honest. `simulate --envtest --compare` will run both and report where they differ: the writes each made, and the children each ended with, after server defaulting and pruning. A clean Blueprint shows no differences, so the fake's fidelity is measured on every run rather than assumed.

## Documentation

A field-by-field reference for every Blueprint block, generated from the same
types the runtime reads.
