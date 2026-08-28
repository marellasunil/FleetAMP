---
title: Rollback
---
# Rollback

Rollback is a new desired-state deployment of an older immutable configuration artifact; FleetAMP does not mutate configuration history.

The target must belong to the same configuration lineage, differ from the current desired configuration, and be older than it. FleetAMP validates the target again before delivery and records the rollback as a deployment action in the audit history.
