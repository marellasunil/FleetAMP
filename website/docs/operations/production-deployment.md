---
title: Production Deployment
---
# Production Deployment

The recommended filesystem direction is `/opt/fleetamp` for binaries, `/etc/fleetamp` for future service configuration, `/var/lib/fleetamp` for persistent state, and `/var/log/fleetamp` for service logs.

Current development examples use HTTP and `ws://`. Production deployments should use controlled network exposure and TLS/WSS. Authentication/RBAC, packaged services, HA and PostgreSQL remain roadmap capabilities and should not be represented as production-ready features yet.

SQLite is appropriate for a single FleetAMP instance. A future multi-replica/active-active deployment requires a shared transactional backend such as PostgreSQL rather than a network-mounted SQLite database.
