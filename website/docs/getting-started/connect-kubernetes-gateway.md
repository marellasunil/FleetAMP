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
