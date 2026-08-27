---
sidebar_position: 1
title: What is FleetAMP?
---

# What is FleetAMP?

FleetAMP is an open-source, vendor-neutral fleet management and configuration control plane for telemetry agents.

The project starts with OpenTelemetry Collectors managed through OpAMP. Its architecture intentionally separates the FleetAMP domain from management protocols, storage engines, Git systems, identity providers, deployment environments, and telemetry backends.

## Project goals

- Maintain a central inventory of managed telemetry agents.
- Track identity, connection state, health, capabilities, and last-seen information.
- Deliver and observe remote configuration through management adapters.
- Support desired-versus-effective configuration state.
- Evolve toward labels, groups, controlled rollouts, Git-driven configuration, RBAC, and enrichment providers.
- Run across VMs, bare metal, containers, and Kubernetes without making the deployment platform part of the core domain.

## Current development focus

FleetAMP is under active development. The initial milestone is intentionally narrow: generic managed-agent modeling, storage abstraction, OpAMP connectivity, inventory APIs, health/state tracking, and remote configuration for OpenTelemetry Collectors.

Features described as planned in these docs are architectural direction and should not be interpreted as implemented functionality.
