---
title: Integrations
---

# Integrations

FleetAMP is designed with integration boundaries from the beginning, while keeping optional integrations out of the initial implementation scope.

## Configuration providers — planned

Configuration providers can connect Git-based workflows such as Azure DevOps, GitHub, GitLab, or local sources to FleetAMP configuration APIs.

## Enrichment providers — planned

Enrichment providers can correlate managed agents with external metadata. A future ServiceNow CSDM/CMDB provider could correlate hosts, services, cloud instances, or CI identifiers and supply business service, application, support group, criticality, or ownership metadata.

## Identity — planned

Authentication can later use OIDC-compatible identity systems. FleetAMP authorization can then apply roles, permissions, and scopes such as platform-wide administration or team-restricted configuration access.

## Additional management adapters — planned

OpenTelemetry Collector via OpAMP is the initial management path. Other telemetry agents can be added through dedicated adapters where their management semantics differ.
