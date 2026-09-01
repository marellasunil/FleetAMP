---
title: OS Deployment and Minimum Requirements
---

# OS deployment and minimum requirements

FleetAMP is a Go application. The core binary, REST API, OpAMP server, SQLite persistence, and structured logging are designed to remain operating-system independent wherever the FleetAMP dependency set can be built and run.

The service manager and log-retention mechanism are platform integrations, not requirements of the FleetAMP core.

## Support status

| Platform | FleetAMP binary | Service integration | Log rotation | Current validation |
| --- | --- | --- | --- | --- |
| Linux | Yes | systemd | logrotate | Locally validated on Fedora |
| macOS | Expected | launchd | platform/external rotation | Guidance only; not yet validated |
| Windows | Expected | Windows service wrapper/scheduler | platform/external rotation | Guidance only; not yet validated |
| Containers/Kubernetes | Yes | container runtime/Kubernetes | runtime/collector policy | Planned packaging; core process model applies |

## Common minimum requirements

FleetAMP does not yet publish benchmark-derived hard minimums. For development and a small single-node installation, use this practical starting baseline:

- 1 CPU core
- 256 MB RAM minimum; 512 MB or more recommended
- 100 MB free disk for the binary and basic runtime files, plus capacity for SQLite state and logs
- TCP connectivity from managed agents to the OpAMP listener (default `4320`)
- access to the HTTP/UI/API listener (default `8080`) from intended administrators or management networks

For source builds, Go **1.24 or newer** is required by `go.mod`, and Git is normally required to clone the repository. A prebuilt FleetAMP binary does not require Go to be installed on the target host.

SQLite is embedded and does not require a separate SQLite server. An OpenTelemetry Collector binary is optional and is only required when `FLEETAMP_OTELCOL_BINARY` is configured for Collector-level configuration validation.

## Common runtime configuration

The same environment variables are used on every operating system:

```text
FLEETAMP_HTTP_ADDR=:8080
FLEETAMP_OPAMP_ADDR=:4320
FLEETAMP_DATA_DIR=<persistent-directory>
FLEETAMP_DATABASE_PATH=<persistent-directory>/fleetamp.db
FLEETAMP_RETIRE_AFTER=24h
FLEETAMP_LOG_LEVEL=info
FLEETAMP_LOG_FORMAT=json
FLEETAMP_LOG_FILE=<log-directory>/fleetamp.log
```

## Linux: systemd and logrotate

Linux is the currently validated long-running service path. Use the reference assets under `deploy/systemd/`.

Recommended locations:

```text
/opt/fleetamp/bin/fleetamp
/etc/fleetamp/fleetamp.env
/var/lib/fleetamp/
/var/log/fleetamp/fleetamp.log
```

Install the systemd unit and environment file, create a dedicated `fleetamp` account, then enable the service with `systemctl enable --now fleetamp`.

The supplied logrotate policy rotates the active file daily or when it reaches the configured size threshold, keeps three backups, removes logs older than three days, and compresses older rotations. FleetAMP also continues to emit the same structured logs to stdout for journald.

Use:

```bash
systemctl status fleetamp
journalctl -u fleetamp -f
```

## macOS: launchd

FleetAMP can use the same binary and environment variables on macOS, but the provided systemd/logrotate files do not apply. Use a `launchd` plist to start the FleetAMP binary at boot/login and restart it when required.

Use macOS paths appropriate to your environment, for example a persistent application-support directory for SQLite state and a dedicated log directory if file logging is enabled. Retention should be handled by a macOS-compatible rotation policy or by forwarding stdout/file logs to an observability collector.

This deployment path is documented as guidance and is not yet part of FleetAMP's validated release matrix.

## Windows

FleetAMP can use the same environment-variable model on Windows. The current Linux systemd unit cannot be reused directly.

Run the FleetAMP executable through an approved Windows service wrapper or another enterprise process-management mechanism that can start the process automatically and restart it on failure. Store persistent state in a service-owned data directory and restrict access to the FleetAMP API and OpAMP ports with Windows/network policy.

Windows log retention should be handled by the selected service/log-management mechanism or by forwarding structured logs to an OpenTelemetry Collector or enterprise logging platform. This path is guidance only until it is tested and packaged by the project.

## Containers and Kubernetes

For containers, prefer structured JSON on stdout. Let Docker, Kubernetes, the node logging layer, or an OpenTelemetry Collector handle collection and retention instead of using systemd or logrotate inside the container.

Persist FleetAMP state on a mounted volume. Do not store SQLite only in the container writable layer if the container may be replaced.

## Production sizing guidance

Resource needs depend mainly on the number of connected agents, configuration activity, deployment history, retained lifecycle data, and API/UI usage. Start small, observe CPU, memory, SQLite growth, and log volume, then increase resources based on measured load.

A single-node SQLite deployment is the current model. Multi-replica/active-active FleetAMP will require a shared transactional backend such as PostgreSQL rather than sharing a SQLite file over a network filesystem.

## Security minimums

For anything beyond a local lab:

- do not expose the HTTP/API listener directly to the public internet
- restrict OpAMP ingress to managed environments
- use TLS/WSS through an appropriate reverse proxy/load balancer until native secure deployment guidance is finalized
- run FleetAMP with a non-root/service account
- protect the persistent data and log directories
- back up persistent state according to the deployment's recovery requirements

Authentication/RBAC and HA are still roadmap capabilities, so the current release should be treated as an early-development control plane rather than a hardened multi-tenant service.
