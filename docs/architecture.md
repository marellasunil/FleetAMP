# FleetAMP Architecture

FleetAMP is designed as a general-purpose, self-hosted fleet-management control plane for telemetry agents.

The project intentionally separates the FleetAMP domain from protocol-, storage-, SCM-, CMDB-, and UI-specific implementations so upstream changes can be absorbed at adapter boundaries.

## Layered architecture

```text
Presentation
  Web UI / REST API
        |
Application
  Fleet, groups, configs, rollout, RBAC
        |
Domain
  ManagedAgent, Group, Config, Deployment, Policy
        |
Integration adapters
  +-- Management: OpAMP today, additional adapters later
  +-- Config: Azure DevOps, GitHub, GitLab, filesystem
  +-- Enrichment: ServiceNow/CMDB, generic REST, other providers
  +-- Identity: OIDC providers
        |
Storage
  Memory -> SQLite -> PostgreSQL
        |
Upstream / external systems
  opamp-go, SCM systems, CMDBs, identity providers
```

## Core design rule

Only the OpAMP integration layer should depend directly on `opamp-go` protocol types. The rest of FleetAMP should use the generic `agents.ManagedAgent` model and normalized management events.

```text
opamp-go / protobufs
        |
internal/opamp
        | translate
        v
ManagedAgent + management.Event
        |
FleetAMP application/domain/storage/UI
```

This allows `opamp-go` to evolve without forcing changes across FleetAMP's grouping, configuration, RBAC, UI, enrichment, or storage layers.

## Managed agent types

The domain model is intentionally broader than OpenTelemetry Collector:

- `otel_collector` — first supported implementation
- `grafana_alloy` — reserved for a future Alloy-specific management adapter after its supported management semantics are implemented and tested
- additional telemetry agents can be added later without changing the core model

Support for an agent type must not be inferred merely because it uses OpenTelemetry components. Each management adapter must explicitly implement the configuration, health, identity, and lifecycle semantics supported by that agent.

## Attributes and labels

FleetAMP keeps reported metadata separate from management metadata:

- **Attributes** are reported by the agent/protocol, for example `host.name`, `os.type`, `service.version`, or `cloud.region`.
- **Labels** are owned by FleetAMP/operators and are used for grouping and targeting, for example `team=payments`, `environment=prod`, or `role=agent`.

External enrichment providers can correlate CMDB/CSDM data and turn approved business context into FleetAMP labels without overwriting raw reported attributes.

## Provider model

Configuration sources implement `providers.ConfigProvider`. This keeps FleetAMP independent of a particular Git vendor.

Examples:

- Azure DevOps
- GitHub
- GitLab
- local filesystem

Enrichment systems implement `providers.EnrichmentProvider`.

Examples:

- ServiceNow CSDM/CMDB
- generic REST CMDB
- organization-specific metadata services

## Storage

FleetAMP application code should depend on storage interfaces rather than a database implementation.

Planned progression:

1. in-memory store for development
2. SQLite for simple self-hosted deployments
3. PostgreSQL for HA and larger deployments

## Initial implementation sequence

1. generic managed-agent domain model
2. storage interfaces and in-memory implementation
3. OpAMP adapter using `opamp-go`
4. Collector inventory and health API
5. remote configuration
6. labels/selectors and groups
7. SQLite persistence
8. config-provider integrations
9. enrichment-provider integrations
10. RBAC/OIDC and rollout controls

## Compatibility philosophy

FleetAMP should prefer composition over forks. Upstream OpenTelemetry components such as `opamp-go`, the OpAMP Supervisor, and `otelcol-contrib` should remain upstream dependencies wherever possible. FleetAMP-specific features should live in independent layers and adapters.
