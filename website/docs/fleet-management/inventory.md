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

A direct OpenTelemetry Collector using the `opamp` extension may report health and effective configuration but not advertise `accepts_remote_config`. FleetAMP displays that distinction rather than treating it as a delivery failure.
