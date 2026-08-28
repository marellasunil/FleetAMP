---
title: Roadmap
---

# Roadmap

FleetAMP is evolving incrementally. This page separates capabilities that are working today from planned work.

## Working now

- Generic `ManagedAgent` domain model
- Storage abstraction with memory, file-backed, and SQLite implementations
- OpAMP server adapter based on `opamp-go`
- Real OpenTelemetry Collector connectivity
- Full-state request after connection
- Health, version, capabilities and descriptive metadata ingestion
- REST inventory API at `/api/v1/agents`
- Basic fleet UI at `/agents`
- Docker Desktop Kubernetes Gateway verified against FleetAMP
- Agent lifecycle: connected, disconnected and retired
- Configurable 24-hour retirement policy
- Persistent agent snapshots and append-only agent event history
- Fleet time-range filtering and `/api/v1/agent-events`
- Agent Details page with desired/effective configuration
- Immutable configuration artifacts with persistent SQLite configuration store
- Persistent SQLite-backed per-agent configuration assignments and delivery status
- OpAMP Supervisor-managed Kubernetes Gateway
- Remote configuration lifecycle verified: `sent -> applying -> applied`
- Pre-deployment configuration validation (YAML plus optional Collector `validate`)
- Desired vs effective configuration drift detection with `in_sync`, `drift`, and `unknown` states
- Safe per-agent rollback to older immutable configuration history with validation and normal OpAMP delivery lifecycle
- Path-level drift diagnostics with desired/effective values
- Immutable configuration version history exposed by configuration name

## Next milestone

- Improve agent identity and deployment metadata for Supervisor-managed Collectors
- Rollback workflow using persisted configuration history
- Continue persistence consolidation by moving agent/event state behind the database abstraction

## Persistence and fleet targeting

- SQLite persistence for configurations and assignments ✅
- PostgreSQL implementation for multi-instance/HA FleetAMP
- Labels, groups and selectors
- Group-based configuration deployment
- Rollout and rollback status

## Integrations and governance

- Git configuration providers
- Azure DevOps / GitHub / GitLab workflows
- Team ownership and scoped authorization
- OIDC authentication and RBAC
- Audit history
- CSDM/CMDB enrichment providers

## Experience and ecosystem

- Rich fleet dashboard
- Pipeline visualization and runtime health
- Additional telemetry-agent management adapters
- Kubernetes and deployment-platform integrations
- HA and autoscaling policy integrations

Roadmap items describe direction and should not be interpreted as production-ready functionality until they move into the working section above.
