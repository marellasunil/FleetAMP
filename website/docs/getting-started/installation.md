---
sidebar_position: 2
title: Installation
---

# Install FleetAMP

FleetAMP is currently an early-development community project. The supported installation path today is to build the binary from source and run it on Linux.

## Requirements

For source builds, FleetAMP requires Go **1.24+**, Git, and network access from managed agents to the OpAMP listener. A prebuilt binary does not require Go or Git on the target host.

A practical small-instance starting baseline is **1 CPU core, 256 MB RAM minimum (512 MB+ recommended), and at least 100 MB free disk plus capacity for runtime state and logs**. These are initial deployment guidelines, not benchmark-derived hard limits.

Linux is the currently validated long-running service platform. FleetAMP core code is intended to remain portable to other operating systems supported by its Go dependency set. See [OS Deployment and Minimum Requirements](../operations/os-deployment.md).

## Production-style filesystem layout

FleetAMP is designed to use standard Linux locations:

```text
/opt/fleetamp/
└── bin/
    └── fleetamp

/etc/fleetamp/
└── fleetamp.env         # systemd environment configuration

/var/lib/fleetamp/       # persistent state (including fleetamp.db)

# logs are written to stdout/stderr and captured by journald
```

## Build and install

```bash
git clone https://github.com/marellasunil/FleetAMP.git
cd FleetAMP
go mod tidy
go test ./...
go build -o fleetamp ./cmd/fleetamp
```

Create the target directory and copy the binary:

```bash
sudo mkdir -p /opt/fleetamp/bin
sudo cp fleetamp /opt/fleetamp/bin/fleetamp
sudo chmod 0755 /opt/fleetamp/bin/fleetamp
```

For a long-running Linux installation, use the reference systemd files under `deploy/systemd/`:

```bash
sudo useradd --system --home-dir /var/lib/fleetamp --shell /sbin/nologin fleetamp
sudo install -d -o fleetamp -g fleetamp /opt/fleetamp/bin /etc/fleetamp /var/lib/fleetamp
sudo install -m 0755 fleetamp /opt/fleetamp/bin/fleetamp
sudo install -m 0644 deploy/systemd/fleetamp.env.example /etc/fleetamp/fleetamp.env
sudo install -m 0644 deploy/systemd/fleetamp.service /etc/systemd/system/fleetamp.service
sudo systemctl daemon-reload
sudo systemctl enable --now fleetamp
```

By default FleetAMP currently listens on:

- HTTP/UI/API: `0.0.0.0:8080`
- OpAMP WebSocket: `0.0.0.0:4320/v1/opamp`

## Verify the service

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/api/v1/agents
```

Open the current fleet page in a browser:

```text
http://localhost:8080/agents
```

## Development run

For local development, installation under `/opt` is not required:

```bash
go run ./cmd/fleetamp
```

## Service logs

FleetAMP writes operational logs to stdout/stderr. A systemd installation captures these logs in journald:

```bash
journalctl -u fleetamp
journalctl -u fleetamp --since "1 hour ago"
journalctl -u fleetamp -f
```

Log retention is managed by the operating system rather than FleetAMP. Configure journald age/size limits according to the host policy.

## Current limitations

FleetAMP now persists configuration artifacts and assignments in embedded SQLite, while agent snapshots and lifecycle events use file-backed persistence. The reference systemd deployment is available and locally validated. Packaged RPM/DEB artifacts, configuration-file loading, TLS termination, authentication, HA/PostgreSQL, and stronger production hardening remain planned work rather than current guarantees.

## Run FleetAMP on another Linux system

For testing, FleetAMP can run directly from a cloned repository. Go is required on that machine:

```bash
git clone https://github.com/marellasunil/FleetAMP.git
cd FleetAMP
go mod tidy
go test ./...
go run ./cmd/fleetamp
```

From another terminal, verify that FleetAMP is listening:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/agents
```

To manage Collectors on other machines or clusters, allow inbound network access to TCP `4320` from those managed environments. The web/API listener on `8080` should only be exposed to intended users or management networks.

## Runtime data and lifecycle settings

For source-based development FleetAMP defaults to `./data`. A Linux service installation should set a persistent location such as:

```bash
export FLEETAMP_DATA_DIR=/var/lib/fleetamp
export FLEETAMP_DATABASE_PATH=/var/lib/fleetamp/fleetamp.db
export FLEETAMP_RETIRE_AFTER=24h
```

`FLEETAMP_DATABASE_PATH` defaults to `<FLEETAMP_DATA_DIR>/fleetamp.db`. FleetAMP uses an embedded, CGo-free SQLite driver, so users do **not** need to install a separate SQLite server or package for normal operation. The database and schema are created automatically on first start.

`FLEETAMP_RETIRE_AFTER` controls how long a disconnected agent remains in the recent fleet before FleetAMP marks it `retired`.

## Structured application logging

For a systemd installation, enable JSON logs in `/etc/fleetamp/fleetamp.env`:

```bash
FLEETAMP_LOG_LEVEL=info
FLEETAMP_LOG_FORMAT=json
FLEETAMP_LOG_FILE=/var/log/fleetamp/fleetamp.log
```

FleetAMP continues to write the same structured records to stdout, so `journalctl -u fleetamp` remains available. The deployment package includes a logrotate policy and timer for daily/10 MB rotation with three retained backups.
