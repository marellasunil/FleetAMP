---
title: Configuration Management
---

# Configuration Management

FleetAMP now includes immutable configuration artifacts, **persistent SQLite-backed configuration and assignment stores**, delivery status, validation, and a capability-gated OpAMP send path.

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

List the version history for one configuration name:

```bash
curl 'http://localhost:8080/api/v1/configurations?name=fleetamp-test.yaml'
```

Because FleetAMP configurations are immutable artifacts, creating a new version does not overwrite an older version. The history endpoint is the same configuration list filtered by exact name and returned newest first. This provides the configuration lineage needed for future rollback workflows.

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

## Safe rollback

Rollback reuses an older immutable configuration artifact; FleetAMP never edits or overwrites configuration history. The rollback endpoint is:

```bash
curl -X POST \
  http://localhost:8080/api/v1/agents/<INSTANCE_UID>/rollback/<TARGET_CONFIG_ID>
```

Before sending anything, FleetAMP verifies that the target exists, belongs to the same configuration name as the agent's current desired configuration, is different from the current artifact, and was created earlier. It then runs the normal pre-deployment validation against the historical content. Only a valid target proceeds to OpAMP delivery.

```text
Current desired v3
      |
      v
Select historical v2
      |
      +-- wrong lineage / same / newer -> reject
      |
      v
Validate YAML + optional Collector validate
      |
      +-- invalid -> reject (HTTP 422)
      |
      v
Create/update assignment -> pending
      |
      v
OpAMP -> sent -> applying -> applied / failed
```

A rollback is therefore a new desired-state assignment to an older immutable artifact. If a previously rolled-back version needs to be promoted forward again, use the normal configuration assignment endpoint rather than the rollback endpoint.


## Persistent desired state

Configuration artifacts and per-agent assignments are stored in the embedded FleetAMP SQLite database. By default the database is created at:

```text
<FLEETAMP_DATA_DIR>/fleetamp.db
```

For a production-style installation you can set:

```bash
export FLEETAMP_DATABASE_PATH=/var/lib/fleetamp/fleetamp.db
```

This means a FleetAMP restart no longer loses the configuration artifact or the record of which configuration was assigned to an agent. When a Supervisor reconnects and reports remote-configuration status, FleetAMP can correlate that status with the persisted assignment. No standalone SQLite installation or database service is required.

Current SQLite tables are `configurations` and `assignments`. Agent snapshots and lifecycle events continue to use the existing file-backed persistence during this incremental migration. PostgreSQL remains a future storage backend for multi-instance/HA FleetAMP deployments.

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

## Desired vs effective configuration drift

FleetAMP compares the latest persisted desired configuration for an agent with the effective configuration reported through OpAMP. Because an OpAMP Supervisor can merge remote configuration with a local/base configuration, FleetAMP uses a semantic YAML subset comparison rather than requiring the complete effective file to be byte-for-byte identical.

Drift states are:

- `in_sync` — every desired setting is present with the same value in the effective configuration
- `drift` — one or more desired settings differ or are missing
- `unknown` — no desired configuration exists, no effective configuration has been reported, or either side cannot be evaluated as YAML

Check drift through the API:

```bash
curl http://localhost:8080/api/v1/agents/<INSTANCE_UID>/drift
```

The Agent Details page also shows the current drift state next to assignment and configuration information. Extra settings in the effective configuration do not count as drift, since they may come from the Supervisor's local/base configuration.

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

## Drift diagnostics

FleetAMP compares the persisted desired configuration with the effective configuration reported through OpAMP. The comparison is YAML-aware and treats the desired configuration as a required subset because a Supervisor can merge FleetAMP remote configuration with local/base configuration.

The drift API is:

```bash
curl http://localhost:8080/api/v1/agents/<INSTANCE_UID>/drift
```

A detected difference now includes the exact YAML path, difference kind, desired value, and effective value. For example:

```json
{
  "status": "drift",
  "in_sync": false,
  "reason": "1 desired setting(s) differ from the reported effective configuration",
  "differences": [
    {
      "path": "service.telemetry.logs.level",
      "kind": "value_mismatch",
      "desired": "info",
      "effective": "debug"
    }
  ]
}
```

Difference kinds currently include `missing`, `value_mismatch`, `type_mismatch`, and `list_length`. If no effective configuration has been reported, FleetAMP returns `unknown` rather than incorrectly reporting drift. The Agent Details page displays these differences and the version history for the desired configuration.
