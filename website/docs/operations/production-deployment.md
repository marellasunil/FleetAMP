---
title: Production Deployment
---
# Production Deployment

For platform-specific prerequisites and service-manager guidance, see [OS Deployment and Minimum Requirements](./os-deployment.md).

FleetAMP now includes a reference Linux systemd deployment under `deploy/systemd/`. The recommended filesystem layout is `/opt/fleetamp` for the binary, `/etc/fleetamp` for service environment configuration, and `/var/lib/fleetamp` for persistent state.

The reference unit runs FleetAMP as a dedicated `fleetamp` user, restarts it after failures, waits for network availability, and applies basic systemd hardening. FleetAMP writes logs to stdout/stderr and systemd captures them in journald.

```bash
systemctl status fleetamp
journalctl -u fleetamp -f
curl http://127.0.0.1:8080/health
```

Log retention should be configured in journald according to host policy using settings such as `SystemMaxUse`, `SystemMaxFileSize`, and `MaxRetentionSec`; FleetAMP does not delete its own service logs.

The reference deployment has been validated on Fedora with the HTTP/API listener on `:8080`, the OpAMP listener on `:4320/v1/opamp`, an OpAMP Supervisor reconnect after restart, and persisted agent state surviving a FleetAMP restart.

Current development examples use HTTP and `ws://`. Production deployments should still use controlled network exposure and TLS/WSS. Authentication/RBAC and HA/PostgreSQL remain roadmap capabilities and should not be represented as production-ready features yet.

SQLite is appropriate for a single FleetAMP instance. A future multi-replica/active-active deployment requires a shared transactional backend such as PostgreSQL rather than a network-mounted SQLite database.

## Application log retention

FleetAMP uses Go structured logging and can write JSON records to `/var/log/fleetamp/fleetamp.log` as well as stdout/journald. The production systemd example creates `/var/log/fleetamp` through `LogsDirectory=fleetamp`.

The supplied `deploy/systemd/fleetamp-logrotate` policy uses `daily`, `maxsize 10M`, `rotate 3`, and `maxage 3`. The optional hourly FleetAMP logrotate timer checks the 10 MB threshold more frequently than the normal daily system logrotate timer. Rotated logs are compressed after one rotation.

Log retention remains an operating-system responsibility; FleetAMP only emits the structured records. This keeps the same logging model usable under systemd, containers, and Kubernetes.

## Operating-system portability

FleetAMP application code is written in Go and the structured logger uses the Go standard library, so the binary and logging behavior can run on operating systems supported by the Go toolchain.

The supplied service-management and rotation files under `deploy/systemd/` are Linux-specific. `systemd` manages the FleetAMP process and `logrotate` manages file retention on Linux distributions that provide those tools.

On other operating systems, keep the same FleetAMP binary and environment variables but use the platform-native service and log-rotation mechanism, for example launchd on macOS or Windows Service management plus a Windows-compatible rotation/collection mechanism.

Runtime databases, snapshots, logs, PID files, and temporary local state are intentionally excluded from Git through `.gitignore`; only reusable deployment definitions and source code belong in the repository.
