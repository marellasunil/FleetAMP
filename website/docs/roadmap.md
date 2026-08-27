---
title: Roadmap
---

# Roadmap

FleetAMP is evolving incrementally. Roadmap items describe direction, not currently available functionality.

## Foundation

- Generic `ManagedAgent` domain model
- Storage abstraction and in-memory implementation
- OpAMP adapter based on `opamp-go`
- Real Supervisor connectivity
- Agent inventory and health/state REST API

## Configuration

- Remote configuration to a single Collector
- Desired vs effective configuration
- Validation and immutable config versions
- Persistent storage
- Labels, groups, and selectors
- Group-based rollout status

## Integrations and governance

- Git configuration providers
- Team ownership and scoped authorization
- OIDC authentication and RBAC
- Audit history
- CSDM/CMDB enrichment providers

## Experience and ecosystem

- Fleet web UI
- Pipeline visualization
- Runtime pipeline health
- Additional telemetry-agent management adapters
- Kubernetes and deployment-platform integrations
- HA and autoscaling policy integrations
