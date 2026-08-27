---
title: Architecture
---

# Architecture

FleetAMP uses a layered architecture so upstream protocol changes and optional integrations do not leak into the core domain.

```text
Presentation
Web UI / REST API
       ↓
Application
Fleet / Groups / Config / Rollouts / Authorization
       ↓
Domain
ManagedAgent / Configuration / Deployment / Policy
       ↓
Adapters and Providers
OpAMP / Git / Enrichment / Identity / future management adapters
       ↓
Storage
Memory / SQLite / PostgreSQL
```

## Management boundary

The first management implementation is OpAMP:

```text
opamp-go
   ↓
internal/opamp
   ↓ translate
FleetAMP domain model
```

OpAMP protobufs should remain inside the OpAMP adapter. Application, API, storage, and UI code should operate on FleetAMP domain types.

## Deployment independence

A managed agent is independent of its runtime. VM, bare metal, container, and Kubernetes information belongs to deployment context and metadata. Kubernetes or cloud platforms remain responsible for scheduling, replicas, and infrastructure lifecycle; FleetAMP focuses on telemetry-agent management state and configuration.

## Future integrations

Provider interfaces are reserved for configuration sources such as Git systems and enrichment sources such as CSDM/CMDB. These are optional integrations, not dependencies of the core.

## Current verified data path

The first working FleetAMP end-to-end path is:

```text
OpenTelemetry Collector Gateway
        │
        │ OpAMP WebSocket
        ▼
internal/opamp
        │ normalize
        ▼
ManagedAgent
        │
        ▼
ManagedAgentStore
        │
        ▼
In-memory store
        │
        ├── GET /api/v1/agents
        └── GET /agents
```

The Collector's OTLP telemetry pipeline is independent of this management connection. FleetAMP manages the Collector control-plane state; it does not sit in the telemetry data path.
