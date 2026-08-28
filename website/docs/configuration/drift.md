---
title: Drift Detection
---
# Drift Detection

FleetAMP compares desired configuration with effective configuration reported by the managed agent. Because a Supervisor can merge a remote fragment with a local base configuration, drift detection uses semantic YAML subset comparison rather than comparing the raw hash of the fragment with the complete effective file.

Drift states are `unknown`, `in_sync`, and `drift`. Diagnostics identify paths and difference kinds such as missing values, value mismatches, type mismatches, and list-length differences.
