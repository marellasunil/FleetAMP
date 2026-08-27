---
sidebar_position: 1
title: What is FleetAMP?
---

# What is FleetAMP?

FleetAMP is an open-source, vendor-neutral fleet management and configuration control plane for telemetry agents.

The first implementation target is the OpenTelemetry Collector managed through OpAMP. FleetAMP separates the core domain from management protocols, storage engines, Git systems, identity providers, deployment environments, and telemetry backends.

## What works today

FleetAMP can accept an OpAMP WebSocket connection from a real OpenTelemetry Collector, normalize that Collector into the `ManagedAgent` model, store it in memory, and expose the resulting fleet inventory through a REST API and basic web UI.

The current verified lab uses an OpenTelemetry Collector gateway running in Docker Desktop Kubernetes and a FleetAMP process running on the Linux host.

## Current endpoints

```text
HTTP / UI / REST API   :8080
OpAMP WebSocket        :4320/v1/opamp
```

## Project direction

Planned milestones include remote configuration, desired/effective state, persistence, groups/selectors, Git-driven workflows, RBAC, CMDB/CSDM enrichment, richer visualization, and additional agent adapters.

Features described as planned are architectural direction and should not be interpreted as implemented functionality.
