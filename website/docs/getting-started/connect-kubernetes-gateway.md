---
sidebar_position: 3
title: Connect OTel Collector to OpAMP
---

# Connect an OTel Collector to FleetAMP with OpAMP

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
        team: payments
        environment: prod
        application: payment-api

service:
  extensions: [opamp]
```

`host.docker.internal` is appropriate for the verified Docker Desktop Kubernetes lab because FleetAMP runs on the host machine. Other Kubernetes environments need a routable FleetAMP address appropriate for that environment.

The values under `agent_description.non_identifying_attributes` are reported metadata. FleetAMP displays them under **Reported OTel attributes**, and they can satisfy group selectors automatically. FleetAMP-managed labels remain a separate operator-owned override layer.

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

FleetAMP requests full OpAMP state after connection so the Collector can report its description, health and effective state. Click the Collector name in `/agents` to open its details page.

For the direct Collector extension used in this lab, `reports_effective_config` indicates that the Collector can report effective configuration state. It does **not** imply that it accepts remote configuration from FleetAMP. If no effective configuration body has been received yet, the detail page explicitly says so.

If you create and assign a FleetAMP configuration to this direct-connected Collector, the expected status is `unsupported` when `accepts_remote_config` is not advertised. Use the OpAMP Supervisor when FleetAMP needs to remotely manage Collector configuration.

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

## Manage the Gateway through the OpAMP Supervisor

Remote configuration requires the Supervisor model rather than the direct Collector `opamp` extension. The verified lab architecture is:

```text
FleetAMP :4320/v1/opamp
        |
        | OpAMP
        v
OpAMP Supervisor 0.149.0
        |
        | launches and manages
        v
OTel Collector Gateway 0.149.0
        |
        +-- OTLP :4317 / :4318
```

The Supervisor advertises `accepts_remote_config`, injects the Collector-side OpAMP extension, merges the local base configuration with FleetAMP remote configuration, restarts the Collector when required, and reports remote-config status and effective configuration back to FleetAMP.

A repeatable lab example is included under `deploy/lab/opamp-supervisor/`. Keep backend credentials out of the repository; use Kubernetes Secrets or environment-specific configuration for real exporters.

### Verified result

A FleetAMP test configuration was assigned to the Supervisor-managed Gateway and progressed through:

```text
sent -> applying -> applied
```

The Agent Details page then displayed the desired artifact and the effective configuration reported back through OpAMP.
