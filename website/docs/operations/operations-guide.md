---
title: Operations Guide
---
# Operations Guide

FleetAMP's default development data directory is `./data`. For a Linux service use persistent storage such as `/var/lib/fleetamp` and set `FLEETAMP_DATABASE_PATH=/var/lib/fleetamp/fleetamp.db`.

Configuration artifacts, assignments, groups and deployment history use embedded SQLite. Agent snapshots and lifecycle events are file-backed. Back up the persistent data directory as one operational unit while FleetAMP is stopped or by using a SQLite-safe backup procedure.

FleetAMP is a management plane, not part of the telemetry data path. A FleetAMP outage should not stop an already-running Collector from sending telemetry to its backend.
