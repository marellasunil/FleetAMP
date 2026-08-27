---
title: Managed Agents
---

# Managed Agents

`ManagedAgent` is FleetAMP's generic representation of a telemetry agent. The domain is deliberately broader than a single OpenTelemetry Collector distribution.

A managed agent can carry identity, agent type, version, connection and health state, last-seen timestamp, capabilities, reported attributes, FleetAMP-owned labels, and deployment context.

## Agent type vs runtime

Agent type describes **what is managed**. Runtime describes **where it runs**.

Examples:

- OpenTelemetry Collector on a VM
- OpenTelemetry Collector in Kubernetes
- A future Grafana Alloy adapter in a container

Keeping these concepts separate prevents FleetAMP from creating a different core model for every platform or agent implementation.
