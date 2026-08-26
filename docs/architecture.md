# FleetAMP Architecture

FleetAMP is designed as a general-purpose, self-hosted fleet-management control plane for telemetry agents.

The project intentionally separates the FleetAMP domain from protocol-, storage-, SCM-, CMDB-, runtime-, and UI-specific implementations so upstream changes can be absorbed at adapter boundaries.

## Layered architecture

```text
Presentation
  Web UI / REST API
        |
Application
  Fleet, groups, configs, rollout, RBAC
        |
Domain
  ManagedAgent, DeploymentContext, Group, Config, Deployment, Policy
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
External runtimes and systems
  VMs, bare metal, containers, Kubernetes, SCM systems, CMDBs,
  identity providers, opamp-go
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

This allows `opamp-go` to evolve without forcing changes across FleetAMP's grouping, configuration, RBAC, UI, enrichment, storage, or runtime model.

## Managed agent types

The domain model is intentionally broader than OpenTelemetry Collector:

- `otel_collector` — first supported implementation
- `grafana_alloy` — reserved for a future Alloy-specific management adapter after its supported management semantics are implemented and tested
- additional telemetry agents can be added later without changing the core model

Support for an agent type must not be inferred merely because it uses OpenTelemetry components. Each management adapter must explicitly implement the configuration, health, identity, and lifecycle semantics supported by that agent.

## Runtime model

Agent type and runtime are separate concerns. The same telemetry agent may run on:

- VM/cloud instance
- bare metal
- standalone container runtime
- Kubernetes

`ManagedAgent.Deployment` uses the generic `DeploymentContext` model so FleetAMP does not become coupled to one cloud or orchestrator.

```text
ManagedAgent
  type: otel_collector | grafana_alloy | ...
  deployment:
    runtime: vm | bare_metal | container | kubernetes
    provider: optional cloud/runtime provider
    cluster: optional cluster name
    namespace: optional namespace
    node: optional node
```

FleetAMP manages telemetry-agent configuration and management state. It does not replace the runtime control plane:

- Kubernetes continues to own Pod scheduling, Deployments, DaemonSets, replicas, autoscaling, and workload lifecycle.
- Cloud/VM automation continues to own instance lifecycle.
- FleetAMP owns agent inventory, management policy, remote configuration, health, targeting, and deployment status.

See `docs/deployment-model.md` for details.

## Attributes and labels

FleetAMP keeps reported metadata separate from management metadata:

- **Attributes** are reported by the agent/protocol, for example `host.name`, `os.type`, `service.version`, `cloud.region`, `k8s.cluster.name`, or `k8s.namespace.name`.
- **Labels** are owned by FleetAMP/operators and are used for grouping and targeting, for example `team=payments`, `environment=prod`, `role=agent`, or `runtime=kubernetes`.

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
2. runtime/deployment context model
3. storage interfaces and in-memory implementation
4. OpAMP adapter using `opamp-go`
5. Collector inventory and health API
6. remote configuration
7. labels/selectors and groups
8. SQLite persistence
9. config-provider integrations
10. enrichment-provider integrations
11. RBAC/OIDC and rollout controls
12. additional management adapters such as Grafana Alloy when their management semantics are explicitly supported

## Compatibility philosophy

FleetAMP should prefer composition over forks. Upstream OpenTelemetry components such as `opamp-go`, the OpAMP Supervisor, and `otelcol-contrib` should remain upstream dependencies wherever possible. FleetAMP-specific features should live in independent layers and adapters.
