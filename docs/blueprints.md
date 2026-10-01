# Blueprints (v0.3.0)

Blueprints provide a guided path from an observability intent to a governed OpenTelemetry Collector deployment. They do not replace FleetAMP configuration versions or deployments; they create the same immutable artifacts used by the existing approval, OpAMP rollout, rollback, drift, and audit workflows.

## Workflow

1. An administrator creates a Destination Profile containing the approved exporter component, endpoint, authentication references, TLS, and environment.
2. A user or Group Owner chooses a Blueprint, reviews its topology and prerequisites, then selects one accessible group and an enabled Destination Profile.
3. FleetAMP generates explicit Collector YAML. Memory limiting and batching are included as governed, locked defaults.
4. FleetAMP validates YAML syntax, Collector structure, and—when `FLEETAMP_OTELCOL_BINARY` is configured—the real Collector distribution.
5. FleetAMP stores the result as an immutable, group-scoped configuration version.
6. The requester may save the version for review or immediately submit it to an eligible reviewer with a change reason and a 7–90 day validity period.
7. Approval uses the existing target snapshot and diff. Approval starts the existing OpAMP deployment; failed delivery invokes automatic rollback to the last successful version.
8. Deployment status, effective configuration, drift, rollback, and audit history remain available in their existing FleetAMP pages.

## Using a Blueprint from a group

Open **Groups & Labels**, select the group, and choose **Use Blueprint** from **Create configuration version**. FleetAMP carries the group into the Blueprint catalog and preselects it after a Blueprint is chosen. The generated version returns to the same group workflow for preview, approval, deployment, rollback and history.

The manual configuration editor starts from the latest successfully applied deployment. Its section tabs, configuration name and version therefore describe the current working baseline. FleetAMP rejects an unchanged copy, a changed configuration that reuses the baseline version, or an existing name/version pair.

Group deployment history shows governed deploy and rollback attempts in every delivery status. Automatic drift reconciliation remains visible in the Audit log instead of being mixed into this user-facing deployment timeline.

## Authorization and safety

- Only administrators manage Destination Profiles.
- Destination Profile exporter configuration is encrypted at rest with authenticated encryption derived from the server secret pepper.
- Group Owners see and target only groups they own or are assigned to.
- Destination internals are not editable in the guided builder.
- A generated artifact must pass current validation again before it enters approval.
- Creating a Blueprint version does not deploy it. A separate eligible reviewer must approve the request.
- Deleting a Destination Profile does not alter existing immutable configuration versions.

## Common Blueprint catalog

| Blueprint | Typical use | Signals | Receiver/topology |
| --- | --- | --- | --- |
| Application APM | Instrumented services | Metrics, traces, logs | OTLP through a Collector gateway |
| Linux Host Baseline | VMs and physical Linux servers | Metrics | Host Collector with host metrics |
| Kubernetes Cluster Baseline | Cluster inventory and health | Metrics | Single cluster-level Collector deployment |
| Central OTLP Gateway | Shared ingestion tier | Metrics, traces, logs | OTLP gRPC and HTTP gateway |
| Application Logs | File-based Linux application logs | Logs | File log receiver on a host Collector |
| Prometheus Metrics | Existing Prometheus-format endpoints | Metrics | Prometheus scrape through a Collector |

Blueprints intentionally exclude exporter credentials, backend endpoints, and TLS policy. Administrators own those values through Destination Profiles. Paths, scrape targets, permissions, resource sizing, and Kubernetes RBAC must be reviewed for the organization before approval.

Additional patterns, reusable modules, profile versioning, and richer policy controls can be added without changing the approval and deployment contract.
