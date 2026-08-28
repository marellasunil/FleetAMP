---
title: Deploy Configuration
---
# Deploy Configuration

FleetAMP sends remote configuration through OpAMP to agents that advertise `accepts_remote_config`. The OpAMP Supervisor is the recommended model for remotely managed OpenTelemetry Collector configuration.

A deployment creates an immutable audit record and progresses through states such as `pending`, `sent`, `applying`, `applied`, `failed`, or `unsupported`. Agent Details shows the current deployed version, latest deployment, duration, last successful deployment, and the latest deployment attempts.

FleetAMP allows only one active deployment per agent while a deployment is pending, sent, or applying. This avoids ambiguous OpAMP status correlation.

## Deployment targeting direction

FleetAMP's targeting model uses named groups rather than duplicating arbitrary label expressions in each deployment. A group has mandatory `application` and `environment` selectors plus optional selectors such as `team`, `region`, or `role`. The same values can be reported by the OTel Collector or supplied as FleetAMP-managed labels.

Group-based deployment is the next rollout layer: FleetAMP will resolve the group's effective membership, snapshot the target agents for auditability, and then reuse the existing per-agent validated OpAMP delivery path.
