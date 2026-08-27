---
title: Configuration Management
---

# Configuration Management

FleetAMP now includes the first configuration-control foundation: immutable configuration artifacts, an in-memory `ConfigurationStore`, per-agent assignments, delivery status, and a capability-gated OpAMP send path.

## Current API

Create a configuration:

```bash
curl -X POST http://localhost:8080/api/v1/configurations \
  -H 'Content-Type: application/json' \
  -d '{"name":"fleetamp-test.yaml","version":"1","content":"service:\n  telemetry:\n    logs:\n      level: info\n","content_type":"text/yaml"}'
```

List configurations:

```bash
curl http://localhost:8080/api/v1/configurations
```

Assign a configuration to one managed agent:

```bash
curl -X POST \
  http://localhost:8080/api/v1/agents/<INSTANCE_UID>/configurations/<CONFIG_ID>
```

List assignment/delivery status:

```bash
curl http://localhost:8080/api/v1/assignments
```

Statuses currently include `pending`, `sent`, `applying`, `applied`, `failed`, and `unsupported`.

## Remote configuration requirement

FleetAMP only sends remote configuration when the managed agent advertises the OpAMP `AcceptsRemoteConfig` capability. This is intentional protocol safety.

The standalone OpenTelemetry Collector `opamp` extension used by the current direct-connection lab does not implement remote configuration. It can report state such as effective config, health, and available components, but FleetAMP will return `unsupported` for a remote-config assignment.

For real remote configuration, run the Collector under the OpenTelemetry OpAMP Supervisor and enable `accepts_remote_config` in the Supervisor configuration. The Supervisor receives the remote config, merges it with local config as applicable, manages the Collector process, and reports remote-config status back to FleetAMP.
