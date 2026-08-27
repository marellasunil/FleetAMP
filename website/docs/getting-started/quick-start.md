---
sidebar_position: 3
title: Quick Start
---

# Quick Start

FleetAMP can now accept an OpAMP connection from a real OpenTelemetry Collector and expose the connected agent through its REST API and basic web UI.

## Prerequisites

- Go version required by `go.mod`
- Git
- An OpenTelemetry Collector with the OpAMP extension for end-to-end testing

## Run from source

```bash
git clone https://github.com/marellasunil/FleetAMP.git
cd FleetAMP
go mod tidy
go test ./...
go vet ./...
go run ./cmd/fleetamp
```

FleetAMP starts two listeners by default:

```text
HTTP / UI / API     :8080
OpAMP WebSocket     :4320/v1/opamp
```

## Verify FleetAMP

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/api/v1/agents
```

Open the fleet view:

```text
http://localhost:8080/agents
```

Before a Collector connects, the page shows an empty fleet. After an OpAMP-capable Collector connects, FleetAMP stores its normalized `ManagedAgent` state in memory.

## Next

- [Install FleetAMP](./installation)
- [Connect a Kubernetes Gateway](./connect-kubernetes-gateway)

