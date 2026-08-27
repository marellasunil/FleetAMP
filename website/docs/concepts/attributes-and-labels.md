---
title: Attributes and Labels
---

# Attributes and Labels

FleetAMP keeps reported metadata separate from management metadata.

## Attributes

Attributes describe facts reported by or derived from the managed agent, for example `host.name`, `os.type`, `service.version`, or cloud and Kubernetes metadata.

## Labels

Labels are FleetAMP-owned management metadata used for organization and future targeting. Examples include `team=payments`, `environment=prod`, and `role=agent`.

This distinction allows future group selectors to target management intent without overwriting facts reported by an agent.

## Enrichment

Future enrichment providers such as CSDM/CMDB integrations should preserve provenance rather than silently replacing reported attributes. Enrichment can later be promoted into management labels or injected into telemetry configuration according to policy.
