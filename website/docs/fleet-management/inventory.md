---
title: Fleet Inventory
---

# Fleet Inventory

Fleet inventory is an initial FleetAMP capability under development.

The target model tracks each managed agent by stable identity and records management state such as version, connected/disconnected status, health, last seen, capabilities, reported attributes, labels, and deployment context.

The initial implementation uses an in-memory store behind a storage interface. Persistent implementations can later use SQLite and PostgreSQL without coupling the OpAMP adapter or REST API to a database engine.
