---
title: Architecture Reference
---
# Architecture Reference

FleetAMP is a control plane. The telemetry data path remains independent:

```text
Application -> OTel Agent -> OTel Gateway -> Observability Backend
                     ^
                     | OpAMP management
                  FleetAMP
```

FleetAMP manages inventory, metadata, groups, desired configuration, deployment state, drift and rollback. It should not become a required hop for application telemetry.

See **Concepts → Architecture** for the detailed management boundary and current verified integration path.
