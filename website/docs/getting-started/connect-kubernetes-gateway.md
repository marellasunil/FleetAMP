---
sidebar_position: 4
title: Connect a Kubernetes Gateway
---

# Connect an OpenTelemetry Gateway

This guide shows the first verified FleetAMP integration path: an OpenTelemetry Collector gateway running in Kubernetes connects to FleetAMP over OpAMP.

## Architecture

```text
Kubernetes
└── OpenTelemetry Gateway
      │
      │ OpAMP WebSocket
      ▼
FleetAMP :4320/v1/opamp
      │
      ├── ManagedAgentStore
      ├── /api/v1/agents
      └── /agents
```

The telemetry path remains independent. OTLP receivers, processors and exporters continue to send telemetry to the configured observability backend.

## Collector OpAMP extension

Add the OpAMP extension to the Collector configuration:

```yaml
extensions:
  opamp:
    server:
      ws:
        endpoint: ws://host.docker.internal:4320/v1/opamp
```

Enable useful reporting capabilities and descriptive metadata:

```yaml
    capabilities:
      reports_effective_config: true
      reports_health: true
      reports_available_components: true
      accepts_restart_command: false

    agent_description:
      non_identifying_attributes:
        collector.role: gateway
        cluster.name: docker-desktop
        deployment.environment: personal-lab

service:
  extensions: [opamp]
```

`host.docker.internal` is appropriate for the verified Docker Desktop Kubernetes lab because FleetAMP runs on the host machine. Other Kubernetes environments need a routable FleetAMP address appropriate for that environment.

## Verify the connection

After applying the Collector configuration, open:

```text
http://localhost:8080/agents
```

or query the REST API:

```bash
curl http://localhost:8080/api/v1/agents
```

A connected gateway should report information such as:

```text
Type        otel_collector
Version     0.149.0
Connected   true
Healthy     true
Runtime     kubernetes
Cluster     docker-desktop
Role        gateway
Environment personal-lab
```

FleetAMP requests full OpAMP state after connection so the Collector can report its description, health and effective state.

## Security

The example uses plain `ws://` only for a local development lab. Production deployments should use TLS (`wss://`), authentication/authorization, and controlled network exposure.

## Connect from another system

If FleetAMP runs on a different server, replace `host.docker.internal` with a DNS name or IP address that the Collector can reach. For example, if the FleetAMP server is `10.10.20.50`:

```yaml
extensions:
  opamp:
    server:
      ws:
        endpoint: ws://10.10.20.50:4320/v1/opamp

service:
  extensions: [opamp]
```

The management path is therefore:

```text
OTel Collector / Gateway
        │
        │ OpAMP :4320
        ▼
FleetAMP server
        │
        ├── http://<fleetamp-server>:8080/agents
        └── http://<fleetamp-server>:8080/api/v1/agents
```

The Collector initiates the OpAMP connection; FleetAMP does not need to connect into the Collector host or Kubernetes pod. Make sure firewalls, security groups, Kubernetes networking, or load balancers allow the Collector to reach the FleetAMP OpAMP listener.

After the Collector connects, verify it from the FleetAMP server:

```bash
curl http://localhost:8080/api/v1/agents
```

Or browse from another permitted workstation to:

```text
http://<fleetamp-server>:8080/agents
```

### Restart behavior in the current milestone

FleetAMP currently stores inventory in memory. Restarting FleetAMP temporarily clears the inventory; OpAMP Collectors reconnect and repopulate it. Persistent storage will remove this limitation in a later milestone.

For production, do not expose the plain `ws://` and HTTP examples directly to untrusted networks. TLS/WSS, authentication, RBAC and hardened network exposure are planned production capabilities.
