# FleetAMP Program Structure

This document explains how the FleetAMP repository is organized and how the main source files participate in the control-plane runtime.

## Runtime flow

```text
OpenTelemetry Collector / OpAMP Supervisor
                |
                | OpAMP WebSocket
                v
        internal/opamp/server.go
                |
                v
        ManagedAgent domain
        internal/agents/
                |
       +--------+---------+----------------+
       |                  |                |
       v                  v                v
    Groups            Configurations   Lifecycle/events
 internal/groups/    internal/configs/ cmd/fleetamp/
       |                  |                |
       +------------------+----------------+
                          |
                          v
                   storage interfaces
                   internal/storage/
                          |
                   +------+------+
                   |             |
                 SQLite       file/memory
                          |
                          v
                 FleetAMP REST API + UI
                   cmd/fleetamp/main.go
```

FleetAMP is the management/control plane. Telemetry continues to flow from applications through OpenTelemetry Collectors to observability backends independently of FleetAMP.

## Repository structure

```text
FleetAMP/
├── cmd/fleetamp/             # Executable, REST API, web UI and runtime orchestration
├── internal/agents/          # Protocol-neutral managed-agent domain
├── internal/configs/         # Configuration, deployment, drift and rollback domain
├── internal/events/          # Agent lifecycle event model
├── internal/groups/          # Controlled group identity and dynamic membership
├── internal/management/      # Protocol-neutral management adapter contract
├── internal/opamp/           # OpenTelemetry OpAMP adapter/server
├── internal/providers/       # Future external config/enrichment provider contracts
├── internal/storage/         # Persistence interfaces and implementations
│   ├── file/                 # JSONL event persistence
│   ├── memory/               # In-memory stores, mainly runtime/tests
│   └── sqlite/               # Default persistent configuration/group database
├── deploy/                   # Deployment/lab assets
├── docs/                     # Repository documentation/assets
├── examples/                 # Example FleetAMP usage/configuration
├── web/                      # Web-related project assets
├── website/                  # Docusaurus documentation website
├── go.mod                    # Go module and direct dependency declarations
├── go.sum                    # Locked Go dependency checksums
└── README.md                 # Project overview and entry documentation
```

## `cmd/fleetamp` — application runtime

| File | Responsibility |
| --- | --- |
| `main.go` | FleetAMP executable entry point. Opens stores, starts OpAMP and HTTP servers, wires configuration delivery/status handling, REST routes and the main Managed Agents UI. |
| `groups.go` | Group and label REST/UI handlers. Implements group CRUD, generated names, enable/disable, agent assignment, membership checks and protected deletion. |
| `lifecycle.go` | Agent lifecycle transitions, retirement loop, snapshot persistence and historical time-range filtering. |
| `deployment_summary_test.go` | Tests the operational deployment summary shown for managed agents. |

## `internal/agents` — managed-agent domain

| File | Responsibility |
| --- | --- |
| `collector.go` | Defines `ManagedAgent`, agent/runtime types, lifecycle status, deployment context, attributes, controlled group fields, labels and capabilities. `Collector` remains a compatibility alias. |
| `registry.go` | Lightweight concurrency-safe in-memory registry retained for compatibility/simple inventory helpers. |
| `server.go` | Reserved extension point for future protocol-independent agent server behavior. |
| `doc.go` | Package-level documentation. |

## `internal/configs` — desired configuration lifecycle

| File | Responsibility |
| --- | --- |
| `configuration.go` | Immutable versioned configuration artifact with content hash and deterministic identity. |
| `assignment.go` | Current desired configuration relationship between an agent and configuration artifact. |
| `deployment.go` | Append-only audit model for deploy and rollback attempts and their timestamps/status. |
| `status.go` | Protocol-neutral remote-configuration status reported back by a management adapter. |
| `validation.go` | YAML validation and optional distribution-aware `otelcol-contrib validate` execution. |
| `drift.go` | Semantic subset comparison between desired and reported effective Collector configuration. |
| `rollback.go` | Safety validation ensuring rollback uses an older artifact from the same configuration lineage. |
| `metadata.go` | Merges `fleetamp.group.*` and `fleetamp.label.*` metadata into `service.telemetry.resource` without replacing unrelated Collector configuration. |
| `*_test.go` | Unit tests for validation, drift and rollback behavior. |
| `doc.go` | Package-level documentation. |

## Groups, events and management adapters

| File | Responsibility |
| --- | --- |
| `internal/groups/group.go` | Group model, enabled state, effective controlled identity, optional labels and exact dynamic membership matching. |
| `internal/groups/group_test.go` | Tests managed/reported identity precedence, label separation and disabled groups. |
| `internal/events/event.go` | Append-only connected/disconnected/retired/health-change event model. |
| `internal/management/adapter.go` | Vendor/protocol-neutral contract FleetAMP core uses to observe agents and deliver remote configuration. |
| `internal/opamp/server.go` | OpAMP WebSocket server/adapter. Converts AgentDescription into ManagedAgent state, parses FleetAMP metadata, caches effective config and sends remote config. |
| `internal/opamp/doc.go` | OpAMP package documentation. |

## `internal/storage` — persistence boundary

The files directly under `internal/storage` define interfaces. Business logic should depend on these contracts rather than directly on SQLite.

| File | Responsibility |
| --- | --- |
| `agent_store.go` | Current ManagedAgent state contract. |
| `config_store.go` | Immutable configuration artifact contract. |
| `assignment_store.go` | Desired assignment and delivery-status contract. |
| `deployment_store.go` | Append-only deployment history contract. |
| `event_store.go` | Agent lifecycle history contract. |
| `group_store.go` | Controlled group persistence contract. |
| `file/event_store.go` | JSONL lifecycle-event persistence (`agent-events.jsonl`). |
| `memory/*.go` | In-memory implementations used by current runtime components and tests. |
| `sqlite/database.go` | Opens FleetAMP SQLite, configures pragmas, initializes/migrates schema and exposes stores. |
| `sqlite/config_store.go` | SQLite configuration artifact persistence. |
| `sqlite/assignment_store.go` | SQLite assignment persistence. |
| `sqlite/deployment_store.go` | SQLite deployment history persistence. |
| `sqlite/group_store.go` | SQLite group selector and enabled-state persistence. |
| `*_test.go` | Store/schema persistence tests. |

## `internal/providers` — external integrations

`config.go` defines a vendor-neutral interface for future configuration sources such as GitHub or Azure DevOps. `enrichment.go` defines a separate metadata-enrichment boundary for future inventory/CMDB integrations. Keeping these interfaces separate prevents external products from becoming part of the FleetAMP core domain.

## Runtime persistence

By default FleetAMP uses `./data` relative to its working directory. Production can set `FLEETAMP_DATA_DIR` and `FLEETAMP_DATABASE_PATH`.

```text
data/
├── fleetamp.db          # SQLite configurations, assignments, deployments and groups
├── fleetamp.db-wal      # SQLite WAL runtime file
├── fleetamp.db-shm      # SQLite shared-memory runtime file
├── agents.json          # Current agent snapshot for restart recovery
└── agent-events.jsonl   # Append-only agent lifecycle history
```

These runtime files are operational state and are not source code that should normally be committed to Git.
