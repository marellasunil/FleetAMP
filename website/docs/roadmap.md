---
title: Roadmap
---

# Roadmap

FleetAMP is evolving incrementally. This page separates capabilities that are working today from planned work.

## Working now

- Generic `ManagedAgent` domain model
- Storage abstraction with in-memory implementation
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
- Immutable configuration artifacts and in-memory configuration store
- Per-agent configuration assignments and delivery status
- OpAMP Supervisor-managed Kubernetes Gateway
- Remote configuration lifecycle verified: `sent -> applying -> applied`
- Pre-deployment configuration validation (YAML plus optional Collector `validate`)

## Next milestone

- Persist agents, configurations and assignments across FleetAMP restarts
- Improve agent identity and deployment metadata for Supervisor-managed Collectors
- Configuration history and drift comparison

## Persistence and fleet targeting

- SQLite persistence
- PostgreSQL implementation
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
