---
title: Fleet Inventory
---

# Fleet Inventory

Fleet inventory is an initial FleetAMP capability under development.

The target model tracks each managed agent by stable identity and records management state such as version, connected/disconnected status, health, last seen, capabilities, reported attributes, labels, and deployment context.

The initial implementation uses an in-memory store behind a storage interface. Persistent implementations can later use SQLite and PostgreSQL without coupling the OpAMP adapter or REST API to a database engine.

## Agent details page

Collector names in the fleet inventory are clickable. The detail page shows:

- identity, version, runtime, cluster, connection and health
- OpAMP capabilities
- latest FleetAMP configuration assignment and delivery status
- desired configuration stored by FleetAMP
- effective configuration reported by the agent when available
- whether the agent advertises remote-configuration support

Current development URL pattern:

```text
/agents/<instance-uid>
```

The page uses the Collector hostname as the preferred human-readable display name when it is available, while the stable OpAMP instance UID remains visible as the management identity.

### Desired versus effective configuration

The detail view deliberately separates two concepts:

- **Desired configuration** is the immutable configuration artifact FleetAMP assigned to the agent.
- **Effective configuration** is the configuration the agent reports it is actually running through OpAMP.

This separation is the foundation for future configuration-drift detection. An agent may advertise `reports_effective_config` without having sent the effective configuration body yet; in that case FleetAMP displays **No effective configuration has been reported yet** rather than assuming the desired configuration is active.

A direct OpenTelemetry Collector using the `opamp` extension may report health/effective-config capability but not advertise `accepts_remote_config`. FleetAMP displays that distinction as `unsupported` rather than treating it as an ordinary delivery failure.

## Agent lifecycle

FleetAMP now tracks an explicit lifecycle for managed agents:

```text
connected
    ↓ connection closes
disconnected
    ↓ after retention period
retired
```

The default retirement period is **24 hours**. Configure it with `FLEETAMP_RETIRE_AFTER`, for example `12h`, `24h`, or `168h`. Retired agents are hidden from the default **Active / recent** view but remain available in **All known** and historical views while their snapshot is retained.

## Time-range fleet view

The `/agents` page includes: Active / recent, Last 15 minutes, Last 1 hour, Last 24 hours, Last 7 days, Last 30 days, and All known. Historical windows are based on FleetAMP agent events recorded during that interval.

Connection history is also available from:

```bash
curl 'http://localhost:8080/api/v1/agent-events?range=24h'
```

## Local persistence

The current milestone uses lightweight file persistence behind the FleetAMP data directory:

```text
<FLEETAMP_DATA_DIR>/agents.json
<FLEETAMP_DATA_DIR>/agent-events.jsonl
```

`agents.json` stores the latest lifecycle snapshot so a disconnected agent can still become retired after a FleetAMP restart. `agent-events.jsonl` is an append-only event history used for time-range queries. This storage is intentionally replaceable; SQLite/PostgreSQL implementations remain planned for larger deployments.
