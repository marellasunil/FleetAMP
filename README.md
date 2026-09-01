# FleetAMP

FleetAMP is a lightweight, self-hosted, open-source fleet management control plane for telemetry agents, with OpenTelemetry Collector and OpAMP as the first implementation target.

> Status: early development / community preview.

FleetAMP is an independent community project and is not an official OpenTelemetry or Grafana project.

## Vision

FleetAMP aims to provide a vendor-neutral management layer for telemetry-agent fleets while keeping protocol, storage, source-control, CMDB, identity, and UI concerns isolated behind stable interfaces.

Initial capabilities are focused on OpenTelemetry Collector through OpAMP. The architecture is intentionally extensible so additional telemetry agents and providers can be added later without rewriting the core platform.

## Goals

FleetAMP aims to provide a simple community-oriented way to:

- discover and inventory managed telemetry agents
- track health, connectivity, version and last-seen status
- identify agents by stable instance identity and useful metadata
- group agents using FleetAMP-owned labels and selectors
- manage versioned configurations
- deploy remote configuration through management adapters
- compare desired and effective configuration
- support safe rollout and rollback workflows
- integrate with Git-based configuration sources
- enrich fleet metadata from CMDB/CSDM or other metadata providers
- add authentication and RBAC without coupling the core to a single identity provider

## Layered architecture

```text
Web UI / REST API
        |
Application services
Fleet / Groups / Config / Rollout / RBAC
        |
Domain model
ManagedAgent / Group / Config / Deployment / Policy
        |
Integration adapters
+-- Management: OpAMP first
+-- Config: Azure DevOps / GitHub / GitLab / filesystem
+-- Enrichment: ServiceNow / generic CMDB / REST
+-- Identity: OIDC providers
        |
Storage abstraction
Memory / SQLite / PostgreSQL
```

The key boundary is:

```text
opamp-go / OpAMP protobufs
          |
    internal/opamp
          | translate
          v
       ManagedAgent
          |
 FleetAMP core / API / UI / storage
```

Only the OpAMP adapter should understand OpAMP-specific wire types. The rest of FleetAMP uses protocol-independent domain models.

See [`docs/architecture.md`](docs/architecture.md) for the detailed design.

## Managed agent model

FleetAMP's core model is `ManagedAgent` rather than a Collector-only type.

Initial and planned agent types include:

- `otel_collector` — first implementation target
- `grafana_alloy` — future adapter; support will be added only after Alloy-specific management semantics are implemented and tested
- additional telemetry agents can be introduced later through new adapters

A compatibility alias named `Collector` remains while the codebase evolves.

## Attributes vs labels

FleetAMP deliberately separates reported metadata from management metadata:

- **Attributes** — reported by the agent/protocol, such as `host.name`, `os.type`, `service.version`, or `cloud.region`
- **Labels** — owned by FleetAMP/operators, such as `team=payments`, `environment=prod`, or `role=agent`

Labels are intended for grouping, policy and deployment targeting. CMDB/CSDM enrichment can add approved business metadata without overwriting raw reported attributes.

## Provider model

FleetAMP uses provider interfaces rather than hard-coding vendors.

Configuration providers can eventually include:

- Azure DevOps
- GitHub
- GitLab
- local filesystem

Enrichment providers can eventually include:

- ServiceNow CSDM/CMDB
- generic REST CMDB
- organization-specific metadata services

## v0.1 scope

The first milestone intentionally stays small:

1. Start the FleetAMP service.
2. Accept OpAMP Supervisor connections.
3. Normalize connected Collectors into the generic `ManagedAgent` model.
4. Track managed agents in memory.
5. Expose inventory through a REST API.
6. Show instance UID, type, name, version, health, connectivity and last seen.
7. Provide a minimal web UI.

Persistence, grouping, remote configuration, rollout control, RBAC, Git integrations and CMDB enrichment will follow later.

## Project structure

```text
FleetAMP/
├── cmd/fleetamp/          # application entry point
├── internal/agents/       # protocol-independent managed-agent domain model
├── internal/management/   # management-adapter contracts
├── internal/opamp/        # OpAMP adapter; opamp-go types stay here
├── internal/providers/    # config/enrichment provider contracts
├── internal/groups/       # fleet grouping and selectors
├── internal/configs/      # versioned configurations
├── internal/api/          # REST API
├── internal/storage/      # persistence abstractions and implementations
├── web/                   # web UI
├── deploy/                # deployment assets, including systemd
├── docs/                  # architecture and project documentation
└── examples/              # sample configurations
```

## Running the current skeleton

Requires Go 1.24+.

```bash
go run ./cmd/fleetamp
```

Then check:

```bash
curl http://localhost:8080/health
```

Expected response:

```json
{"status":"ok","service":"fleetamp"}
```

## Running as a Linux service

FleetAMP includes a reference systemd deployment under [`deploy/systemd/`](deploy/systemd/). The service uses a dedicated `fleetamp` account, `/opt/fleetamp/bin/fleetamp` for the binary, `/etc/fleetamp/fleetamp.env` for environment configuration, and `/var/lib/fleetamp` for persistent state.

FleetAMP writes operational logs to stdout/stderr. When run with systemd these logs are captured by journald:

```bash
journalctl -u fleetamp
journalctl -u fleetamp -f
```

See [`deploy/systemd/README.md`](deploy/systemd/README.md) for build, installation, verification, upgrade, and log-retention guidance.

For OS-specific deployment models and initial sizing requirements, see [`website/docs/operations/os-deployment.md`](website/docs/operations/os-deployment.md). Linux/systemd is currently validated; macOS/Windows guidance is documented but not yet release-tested.

## Roadmap

- **v0.1** — OpAMP server adapter, managed-agent inventory, health, REST API, memory store, basic UI
- **v0.2** — SQLite persistence, remote configuration, config history, desired-vs-effective state
- **v0.3** — label-based groups, group deployment, rollout/rollback
- **v0.4** — PostgreSQL, OIDC/RBAC, audit and HA-oriented deployment
- **v0.5+** — canaries, drift detection, package upgrades, runtime telemetry visualization, Helm/Kubernetes deployment, additional agent adapters

## Design principles

- Prefer composition over forks.
- Keep upstream protocol types at integration boundaries.
- Keep the domain model vendor-neutral.
- Separate desired state from observed runtime state.
- Make source-control, database, identity and CMDB integrations replaceable.
- Add support for new telemetry agents through explicit adapters rather than assumptions about config compatibility.

## License

Apache License 2.0.

## Upstream projects

FleetAMP intends to build on and interoperate with the OpenTelemetry ecosystem, especially:

- OpenTelemetry OpAMP specification
- `open-telemetry/opamp-go`
- OpenTelemetry OpAMP Supervisor
- OpenTelemetry Collector / `otelcol-contrib`

## Program structure

For a source-code map and explanation of how FleetAMP components fit together, see [`docs/PROGRAM_STRUCTURE.md`](docs/PROGRAM_STRUCTURE.md).
