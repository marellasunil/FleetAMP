---
sidebar_position: 2
title: Installation
---

# Install FleetAMP

FleetAMP is currently an early-development community project. The supported installation path today is to build the binary from source and run it on Linux.

## Requirements

- Linux x86_64 or another platform supported by Go
- Go version required by `go.mod`
- Git
- Network access from managed agents to the FleetAMP OpAMP listener

## Production-style filesystem layout

FleetAMP is designed to use standard Linux locations:

```text
/opt/fleetamp/
└── bin/
    └── fleetamp

/etc/fleetamp/
└── fleetamp.yaml        # future configuration file

/var/lib/fleetamp/       # future persistent state
/var/log/fleetamp/       # future service logs
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

Run FleetAMP:

```bash
/opt/fleetamp/bin/fleetamp
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

## Current limitations

The current implementation uses an in-memory agent store. Fleet inventory is therefore lost when FleetAMP restarts. Persistent storage, packaged RPM/DEB artifacts, a systemd unit, configuration-file loading, TLS termination, authentication, and production hardening are planned work rather than current guarantees.

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
export FLEETAMP_RETIRE_AFTER=24h
```

`FLEETAMP_RETIRE_AFTER` controls how long a disconnected agent remains in the recent fleet before FleetAMP marks it `retired`.
