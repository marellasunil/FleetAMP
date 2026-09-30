# Blueprints (v0.3.0)

Blueprints provide a guided path from an observability intent to a governed OpenTelemetry Collector deployment. They do not replace FleetAMP configuration versions or deployments; they create the same immutable artifacts used by the existing approval, OpAMP rollout, rollback, drift, and audit workflows.

## Workflow

1. An administrator creates a Destination Profile containing the approved exporter component, endpoint, authentication references, TLS, and environment.
2. A user or Group Owner chooses a Blueprint, one accessible group, the required signals, and an enabled Destination Profile.
3. FleetAMP generates explicit Collector YAML. v0.3.0 includes **OTLP application service** and **Host observability** patterns. Memory limiting and batching are included as governed defaults.
4. FleetAMP validates YAML syntax, Collector structure, and—when `FLEETAMP_OTELCOL_BINARY` is configured—the real Collector distribution.
5. FleetAMP stores the result as an immutable, group-scoped configuration version.
6. The requester may save the version for review or immediately submit it to an eligible reviewer with a change reason and a 7–90 day validity period.
7. Approval uses the existing target snapshot and diff. Approval starts the existing OpAMP deployment; failed delivery invokes automatic rollback to the last successful version.
8. Deployment status, effective configuration, drift, rollback, and audit history remain available in their existing FleetAMP pages.

## Authorization and safety

- Only administrators manage Destination Profiles.
- Group Owners see and target only groups they own or are assigned to.
- Destination internals are not editable in the guided builder.
- A generated artifact must pass current validation again before it enters approval.
- Creating a Blueprint version does not deploy it. A separate eligible reviewer must approve the request.
- Deleting a Destination Profile does not alter existing immutable configuration versions.

## Initial patterns

| Pattern | Signals | Receiver | Governed processors |
| --- | --- | --- | --- |
| OTLP application service | Metrics, traces, logs (selectable) | OTLP gRPC and HTTP | Memory limiter, batch |
| Host observability | Metrics | Host metrics | Memory limiter, batch |

Additional patterns, reusable modules, profile versioning, and richer policy controls can be added without changing the approval and deployment contract.
