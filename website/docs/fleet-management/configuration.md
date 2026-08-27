---
title: Configuration Management
---

# Configuration Management

Configuration management is a planned core capability following stable OpAMP inventory and state tracking.

The design separates configuration content, immutable versions, assignment/targeting, desired state, effective state, and deployment result.

A future group rollout can follow this path:

```text
Validated configuration
        ↓
FleetAMP desired state
        ↓
Group / selector
        ↓
Management adapter
        ↓
Managed agents
        ↓
Effective status reported back to FleetAMP
```

Git integrations such as Azure DevOps, GitHub, or GitLab are expected to call vendor-neutral FleetAMP configuration APIs rather than becoming requirements of the core service.
