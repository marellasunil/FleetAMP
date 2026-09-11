# FleetAMP

FleetAMP is a lightweight, self-hosted, open-source fleet management control plane for telemetry agents, with OpenTelemetry Collector and OpAMP as the first implementation target.

> Status: early development / community preview.

FleetAMP is an independent community project and is not an official OpenTelemetry or Grafana project.

Documentation: [fleetamp.marellasunil.com](https://fleetamp.marellasunil.com)

Documentation source: [marellasunil/FleetAMP-docs](https://github.com/marellasunil/FleetAMP-docs)

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

See the [detailed architecture documentation](https://fleetamp.marellasunil.com/docs/development/internal-architecture).

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

## v0.1.0 community preview

FleetAMP v0.1.0 is the first packaged community preview. It is suitable for
evaluation and a controlled single-server pilot, but it is not yet a supported
production release.

The preview includes persistent OpAMP agent inventory, Active/Offline/Retired
lifecycle state, stable logical-agent reassociation, SQLite-backed groups and
assignments, versioned configuration deployment and history, rollback, first-login
administrator setup, session security, optional TLS and mTLS, and Linux systemd
deployment assets.

Known gaps include high availability, PostgreSQL, OIDC/RBAC, approval separation,
certificate-to-agent authorization, Collector binary upgrades, and production
scale certification. These remain pre-v1.0 work.

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
├── deploy/                # runtime deployment assets, including systemd
├── scripts/               # installation and upgrade automation
├── go.mod                 # Go module definition
└── go.sum                 # locked dependency checksums
```

## Install a release

End users do not need to clone this repository or install Go. Download the archive
for the laptop or server architecture from
[GitHub Releases](https://github.com/marellasunil/FleetAMP/releases), download
`SHA256SUMS`, and verify the archive before extracting it:

```bash
sha256sum -c SHA256SUMS
tar -xzf fleetamp_0.1.0_linux_amd64.tar.gz
cd fleetamp_0.1.0_linux_amd64
./fleetamp --version
```

Use `linux_arm64` instead on a 64-bit ARM system. The archive includes the
systemd examples and `scripts/install-user.sh`; read
[`deploy/systemd/README.md`](deploy/systemd/README.md) before installing.

## Build from source (contributors)

Building from source requires Go 1.25+.

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

## Security

FleetAMP listens on localhost by default. The first administrator is created
through a one-time `/setup` flow. Passwords are stored as Argon2id verifiers
bound to a per-server pepper, which can be protected by TPM-backed systemd
credentials. Remote OpAMP access requires a bearer token unless
`FLEETAMP_ALLOW_INSECURE=true` is explicitly set for an isolated development
environment. Protect the web UI/API and OpAMP traffic with FleetAMP native
TLS or explicit TLS termination at a trusted proxy/load balancer. Optional
OpAMP mTLS verifies agent client certificates. Never commit credentials or
private keys. See the
[security hardening guide](https://fleetamp.marellasunil.com/docs/operations/security-hardening)
and [TLS guide](https://fleetamp.marellasunil.com/docs/operations/transport-tls).

Security checks run tests, the race detector, `go vet`, and `govulncheck` in
GitHub Actions. Dependabot monitors Go modules and workflow actions.

For upgrading the upstream OpAMP Go dependency, see the [OpAMP upgrade guide](https://fleetamp.marellasunil.com/docs/operations/opamp-upgrade).

For OS-specific deployment models and initial sizing requirements, see the [OS deployment guide](https://fleetamp.marellasunil.com/docs/operations/os-deployment). Linux/systemd is currently validated; macOS/Windows guidance is documented but not yet release-tested.

## Roadmap

- **v0.1.x** — community preview packaging, single-node pilot validation, hardening and bug fixes
- **v0.2** — approval workflow, audit trail, RBAC foundation and rollout safeguards
- **v0.3** — PostgreSQL, high-availability foundations and broader scale testing
- **v0.4+** — package upgrades, drift detection, canaries, Helm/Kubernetes deployment and additional agent adapters
- **v1.0** — stable production baseline with documented compatibility, upgrade and support expectations

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

For a source-code map and explanation of how FleetAMP components fit together, see the [program structure guide](https://fleetamp.marellasunil.com/docs/development/program-structure).
