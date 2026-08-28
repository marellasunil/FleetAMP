---
title: Managed Collectors
---
# Managed Collectors

After an OpenTelemetry Collector or Supervisor connects over OpAMP, FleetAMP records it as a managed agent. Open **Agents** to see collector type, version, OS/architecture, runtime, connection status, health, latest deployment state and last-seen time.

Open a collector to inspect its identity, capabilities, reported OTel attributes, FleetAMP-managed labels, matching groups, desired/effective configuration, drift and deployment history.

## Lifecycle

FleetAMP tracks `connected`, `disconnected` and `retired` states. A restart restores known agents from the persistent snapshot as disconnected until they reconnect. `FLEETAMP_RETIRE_AFTER` controls retirement timing.
