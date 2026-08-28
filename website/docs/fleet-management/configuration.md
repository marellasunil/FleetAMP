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


## Pre-deployment validation

FleetAMP validates configuration before it is stored as desired state and validates it again immediately before assignment. This creates a safety gate in front of OpAMP remote configuration.

```text
Configuration
    |
    v
YAML syntax validation
    |
    +-- invalid -> reject (HTTP 422)
    |
    v
Optional OTel Collector `validate` command
    |
    +-- invalid -> reject (HTTP 422)
    |
    v
Store / assign / send through OpAMP
```

YAML validation is always enabled. To additionally validate against the exact Collector distribution installed on the FleetAMP host, configure:

```bash
export FLEETAMP_OTELCOL_BINARY=/opt/otelcol-contrib/otelcol-contrib
```

FleetAMP then runs the Collector's `validate --config=<temporary-file>` command with a bounded timeout. This checks the configuration against the actual receivers, processors, exporters, extensions, connectors, and component-specific settings available in that Collector distribution. The temporary validation file is removed after validation.

A configuration can be checked without creating it:

```bash
curl -X POST http://localhost:8080/api/v1/configurations/validate \
  -H 'Content-Type: application/json' \
  -d '{"content":"service:\n  telemetry:\n    logs:\n      level: info\n"}'
```

When no Collector binary is configured, a successful response reports `yaml_valid: true`, `collector_skipped: true`, and `valid: true`. This means the YAML is structurally valid, but distribution-aware validation has not been performed.

## Remote configuration requirement

FleetAMP only sends remote configuration when the managed agent advertises the OpAMP `AcceptsRemoteConfig` capability. This is intentional protocol safety.

The standalone OpenTelemetry Collector `opamp` extension used by the current direct-connection lab does not implement remote configuration. It can report state such as effective config, health, and available components, but FleetAMP will return `unsupported` for a remote-config assignment.

For real remote configuration, run the Collector under the OpenTelemetry OpAMP Supervisor and enable `accepts_remote_config` in the Supervisor configuration. The Supervisor receives the remote config, merges it with local config as applicable, manages the Collector process, and reports remote-config status back to FleetAMP.

## Agent details and delivery visibility

Open `/agents`, then select a Collector name to inspect its management state. The detail page correlates the latest assignment with the desired configuration artifact and the latest effective configuration reported through OpAMP.

For a direct Collector connection that does not advertise `AcceptsRemoteConfig`, an assignment is retained for audit/visibility but its delivery status becomes:

```text
unsupported
agent does not advertise accepts_remote_config
```

This is different from `failed`: `unsupported` means FleetAMP intentionally did not attempt an unsafe or unsupported protocol operation.

### Configuration lifecycle

```text
Configuration artifact
        │
        ▼
Per-agent assignment
        │
        ├── unsupported   agent cannot accept remote config
        └── sent          FleetAMP offered the config through OpAMP
                │
                ├── applying
                ├── applied
                └── failed

Agent-reported effective config ──► Agent Details page
```

With an OpAMP Supervisor, the intended lifecycle is **desired → sent → applying → applied**, with the effective configuration providing evidence of what the managed Collector is actually running.
