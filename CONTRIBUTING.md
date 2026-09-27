# Contributing to FleetAMP

Thanks for considering a contribution.

## Development flow

1. Fork or branch from `main`.
2. Keep changes focused and small.
3. Run `go test ./...` before opening a pull request.
4. Document user-facing behavior changes.
5. Avoid committing credentials, tokens, certificates, or private keys.

## Current contribution areas

FleetAMP v0.2 establishes the governed fleet-management loop: inventory,
groups, Group Owners, configuration validation, approval, deployment,
rollback, drift and audit. Useful contribution areas now include:

- regression tests, security hardening and accessibility;
- OpenTelemetry Collector and Supervisor compatibility testing;
- documentation and reproducible Linux or container-based labs;
- instrumentation guides and reusable telemetry blueprints planned for v0.3;
- design proposals for storage, identity and deployment integrations.

Blueprints, AI Insights and MCP are visible roadmap areas, not production
capabilities. Please open a design issue before implementing a large new
subsystem or changing a security or governance boundary.

## Pull requests

Please describe the problem, the proposed change, how it was tested, and any compatibility considerations with OpenTelemetry or OpAMP.

By submitting a contribution, you agree that it is licensed under the
repository's Apache License 2.0. See [GOVERNANCE.md](GOVERNANCE.md) for the
current decision process and project roles.
