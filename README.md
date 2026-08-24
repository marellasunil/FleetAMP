# FleetAMP

FleetAMP is a lightweight, self-hosted, open-source fleet management control plane for OpenTelemetry Collectors, powered by OpAMP.

> Status: early development / community preview.

## Goals

FleetAMP aims to provide a simple community-oriented way to:

- discover and inventory OpenTelemetry Collectors
- track Collector health and last-seen status
- identify Collectors by stable instance UID and useful metadata
- group Collectors by labels such as team, environment, region, and role
- manage versioned Collector configurations
- deploy remote configuration through OpAMP
- compare desired and effective configuration
- support safe rollout and rollback workflows

## Initial architecture

```text
Web UI / REST API
        |
        v
FleetAMP control plane
        |
        +--> Fleet state / storage
        |
        v
     opamp-go
        |
        v
OpAMP Supervisors
        |
        v
otelcol-contrib
```

## v0.1 scope

The first milestone intentionally stays small:

1. Start the FleetAMP service.
2. Accept OpAMP Supervisor connections.
3. Track connected Collectors in memory.
4. Expose Collector inventory through a REST API.
5. Show instance UID, hostname, version, health, and last seen.
6. Provide a minimal web UI.

Persistence, grouping, remote configuration, rollout control, RBAC, and GitOps integration will follow in later milestones.

## Project structure

```text
FleetAMP/
├── cmd/fleetamp/          # application entry point
├── internal/opamp/        # OpAMP server integration
├── internal/agents/       # Collector inventory and state
├── internal/groups/       # fleet grouping and selectors
├── internal/configs/      # versioned Collector configs
├── internal/api/          # REST API
├── internal/storage/      # persistence abstraction
├── web/                   # web UI
├── docs/                  # project documentation
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

## Roadmap

- **v0.1** — Collector inventory, health, REST API, basic UI
- **v0.2** — SQLite persistence, groups, config versions, remote config
- **v0.3** — rollout/rollback, desired-vs-effective state, PostgreSQL option
- **v0.4+** — OIDC/RBAC, canaries, drift detection, runtime telemetry, Helm deployment

## License

Apache License 2.0.

## Upstream projects

FleetAMP is intended to build on the OpenTelemetry ecosystem, especially:

- OpenTelemetry OpAMP specification
- `open-telemetry/opamp-go`
- OpenTelemetry OpAMP Supervisor
- OpenTelemetry Collector / `otelcol-contrib`

FleetAMP is an independent community project and is not an official OpenTelemetry project.
