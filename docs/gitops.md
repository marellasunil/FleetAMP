# Governed GitOps delivery

FleetAMP accepts change intent, resolves authorized targets, validates
compatibility, creates an immutable repository preview and requires approval
before an execution is queued. It does not create repositories or pipelines.

## How the workflow fits together

| Step | Record | Purpose |
| --- | --- | --- |
| 1. Register | Repository connection | Defines provider, repository, branch, allowed root and permitted groups |
| 2. Validate | Connection validation | Proves current repository and branch access for a limited time |
| 3. Propose | Component lifecycle request | Freezes operation, component, target scope, method and versions |
| 4. Resolve | Compatibility validation | Snapshots eligible targets and blocking findings |
| 5. Approve | Component approval | Applies four-eyes review to exact proposal evidence |
| 6. Prepare | Execution plan and GitOps preview | Resolves the safe path and renders exact files and diff |
| 7. Approve preview | GitOps preview approval | Authorizes the exact preview and hashes |
| 8. Queue | Execution outbox request | Creates immutable provider work for the worker |
| 9. Deliver | Append-only execution events | Records dry-run or GitHub pull-request progress |
| 10. Observe | Statistics and audit | Exposes status, retries, actors and outcomes |

## 1. Configure a credential reference

The UI stores only a reference such as:

```text
secret://github/production
```

FleetAMP resolves that reference at runtime from the corresponding environment
variable:

```bash
FLEETAMP_GITOPS_SECRET_GITHUB_PRODUCTION=replace-with-test-token
```

Do not put the token in a repository connection, preview, configuration file or
Git commit. For GitHub pull-request delivery, use a fine-grained token limited
to the selected test repository with Contents read/write and Pull requests
read/write permissions.

## 2. Register and validate a connection

1. Open **Administration → Integrations**.
2. Enter a connection name and select the provider first.
3. Complete only the fields displayed for GitHub, GitLab or Azure DevOps.
4. Select an existing repository and branch.
5. Set a relative allowed root such as `fleetamp/groups`.
6. Select the groups permitted to use the connection.
7. Enter the `secret://` credential reference and save.
8. Select **Test connection**.

Validation evidence expires after 24 hours by default. Override this with
`FLEETAMP_INTEGRATION_VALIDATION_TTL`. Self-hosted provider hosts must be HTTPS
and explicitly listed in `FLEETAMP_INTEGRATION_ALLOWED_HOSTS`.

## 3. Create and approve a component proposal

1. Open **Deployments**.
2. Select the operation and component type.
3. Select the ownership group and optional exact-match labels.
4. Select **GitOps** as the delivery method.
5. Supply only the versions required for the operation.
6. Explain the reason and create the immutable proposal.
7. Select **Resolve targets & validate**.
8. Resolve blocking compatibility findings.
9. Assign a different eligible reviewer and submit for approval.
10. Have the assigned reviewer approve the exact request.

Creating or approving a proposal does not modify a repository.

## 4. Prepare and approve the GitOps preview

1. Open **Previews & approvals** from the component approval or GitOps pages.
2. Select the approved plan and a currently validated connection.
3. Generate the preview.
4. Verify the repository path is below the allowed root.
5. Review every generated file and the diff.
6. Assign a reviewer and submit the exact preview.
7. Have that reviewer approve it.
8. Queue the approved preview for execution.

FleetAMP binds the approval to the plan hash, preview hash, connection snapshot,
branch and repository path. A changed preview requires a new approval.

## 5. Run in dry-run mode

Dry-run is the recommended initial configuration:

```bash
FLEETAMP_GITOPS_WORKER_ENABLED=true
FLEETAMP_GITOPS_GITHUB_WRITES_ENABLED=false
```

The worker validates and records the provider execution without committing or
opening a pull request. GitHub, GitLab and Azure DevOps can all use this path.

## 6. Enable optional GitHub pull-request delivery

Only enable writes after the dry-run E2E flow succeeds against a dedicated test
repository:

```bash
FLEETAMP_GITOPS_WORKER_ENABLED=true
FLEETAMP_GITOPS_GITHUB_WRITES_ENABLED=true
FLEETAMP_GITOPS_GITHUB_STATUS_SYNC_ENABLED=true
```

Optional tuning:

```bash
FLEETAMP_GITOPS_WORKER_ID=fleetamp-worker-1
FLEETAMP_GITOPS_WORKER_INTERVAL=5s
FLEETAMP_GITOPS_LEASE_TTL=30s
FLEETAMP_GITOPS_STATUS_SYNC_ID=fleetamp-worker-1-status
FLEETAMP_GITOPS_STATUS_SYNC_INTERVAL=30s
```

Restart FleetAMP after changing service environment variables. GitLab and Azure
DevOps delivery remain dry-run in v0.3.0.

## 7. Monitor and recover

Open **GitOps execution monitor** to view the append-only execution timeline.
The worker uses a lease, allowing another worker to reclaim an interrupted item
after expiry. A failed execution can be retried by an administrator with a
reason, up to three total attempts. Recovery reuses the same execution ID and
deterministic branch to avoid duplicate pull requests.

Use **Audit Log → GitOps & Integrations** to trace connection registration,
validation, preview, approval, queueing, retry and completion.

## Provider support in v0.3.0

| Provider | Connection validation | Dry-run execution | Repository writes |
| --- | --- | --- | --- |
| GitHub | Yes | Yes | Pull request, explicitly enabled |
| GitLab | Yes | Yes | Not enabled |
| Azure DevOps | Yes | Yes | Not enabled |

## Security checklist

- Use a dedicated repository and least-privilege credential for testing.
- Keep the worker and GitHub writes disabled by default.
- Restrict every connection to explicit FleetAMP groups.
- Use a relative allowed root and review the resolved preview path.
- Never approve your own proposal or preview.
- Revalidate expired repository evidence before queueing.
- Confirm audit evidence contains references and hashes, not raw credentials.

Use the [v0.3.0 E2E checklist](e2e-v0.3.0.md) before enabling real writes in a
release candidate.
