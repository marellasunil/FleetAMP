# Configuration migration

Migration converts an existing OpenTelemetry Collector configuration into a
governed FleetAMP group configuration version. It does not deploy the imported
configuration automatically.

## Before you begin

- Create or identify the destination ownership group.
- Confirm you can create configuration versions for that group.
- Remove literal passwords and tokens from the source YAML. Use
  [group-secret references](group-secrets.md) instead.
- Keep a copy of the original configuration for comparison.

## Workflow

| Stage | What FleetAMP does | What the user checks |
| --- | --- | --- |
| Import | Accepts pasted or uploaded Collector YAML | Correct source configuration |
| Parse | Reads the Collector document and its component sections | YAML syntax and recognized sections |
| Normalize | Produces a consistent internal representation | No intended behavior was lost |
| Standardize | Applies governed recommendations and reports changes | Every added, changed or retained item |
| Pattern match | Compares the configuration with reusable patterns | Suggested pattern is appropriate |
| Validate | Runs FleetAMP policy and optional Collector validation | No blocking errors remain |
| Preview | Shows the final content before persistence | Preview matches the intended result |
| Save | Creates an immutable group configuration version | Group, version identity and content |
| History | Preserves previous migration results | Actor, time and result are traceable |
| Optional approval | Submits the exact saved version | Reviewer and comment are correct |

## Step-by-step

1. Open **Migration** from the FleetAMP navigation.
2. Select the target group.
3. Paste the existing Collector YAML or upload the configuration file.
4. Parse and normalize it.
5. Review FleetAMP's standardization findings. A recommendation is not a
   deployment; confirm it matches the organization's policy.
6. Review any recognized governed pattern and adoption impact.
7. Run **Validate**. Resolve all blocking findings.
8. Open the final **Preview** and compare it with the source configuration.
9. Save the preview as a new immutable group configuration version.
10. Open **Migration History** and confirm the saved result.
11. When the configuration is ready for rollout, optionally submit that exact
    version to the existing approval workflow.

## Validation levels

FleetAMP always validates YAML and its governance rules. When
`FLEETAMP_OTELCOL_BINARY` points to the intended Collector executable, FleetAMP
can also validate distribution-specific components and pipeline settings.

A configuration that passed previously can be rejected at the approval
boundary if policy or the validation binary changed. This prevents stale
validation evidence from authorizing a deployment.

## Safety properties

- Saving creates a new immutable version; it does not overwrite an older one.
- Preview happens before persistence.
- Deployment requires the normal approval and authorization checks.
- Migration history is retained for traceability.
- Literal credentials should never be stored in imported YAML.
- A Group Owner remains limited to assigned groups.

## Common failures

| Problem | Resolution |
| --- | --- |
| YAML cannot be parsed | Correct indentation, mappings and sequences in the source |
| Component is unknown | Validate with the same Collector distribution used by the target |
| Pipeline references a missing component | Add the component or remove the invalid reference |
| Secret value appears in YAML | Replace it with `${secret:key}` and create the group secret |
| Save is rejected as unchanged | Use the existing version or make an intentional change |
| Approval is blocked after saving | Rerun validation under the current policy |

Use the [v0.3.0 E2E checklist](e2e-v0.3.0.md) to qualify this workflow for a
release.
