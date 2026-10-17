# FleetAMP v0.3.0 end-to-end validation

This checklist verifies the user-visible v0.3.0 workflows after the automated
release gate passes. Record the result and evidence for every scenario before
tagging the release.

## 1. Test boundaries

- Use a disposable FleetAMP database or a copied v0.2 database.
- Keep the existing service on `localhost:8080` unchanged when using a separate
  test instance.
- Use test users, groups, agents and repositories.
- Never use a production repository token or the only copy of a database.
- Keep Git provider writes disabled until the dry-run workflow succeeds.

## 2. Prerequisites

| Requirement | Verification |
| --- | --- |
| Clean release branch | `git status --short --branch` |
| Automated release gate passed | `bash scripts/verify-v0.3.0.sh` |
| FleetAMP is healthy | `curl --fail http://127.0.0.1:8080/health` |
| FleetAMP is ready | `curl --fail http://127.0.0.1:8080/ready` |
| Administrator account exists | Sign in and open Administration |
| Test ownership group exists | Open **Groups & Labels** |
| At least three test Collectors exist | Keep two unassigned and one assigned for membership scenarios |
| Approved group configurations exist | Prepare source and destination versions for move/restore tests |
| Test Git repository exists | Use an empty or dedicated non-production repository |

## 3. Record the test run

| Field | Value |
| --- | --- |
| Date | |
| Tester | |
| Commit | |
| FleetAMP version | |
| Database source | Fresh / copied v0.2 |
| Git provider mode | Dry-run / GitHub PR |
| Result | Pass / Fail |

## 4. Core regression

### 4.1 Authentication and authorization

1. Sign in as an administrator.
2. Change the administrator password from the account page.
3. Create or use a Group Owner assigned to only the test group.
4. Sign in as that Group Owner and confirm unrelated groups and Admin pages are
   not accessible.

Expected result: password changes preserve access, sessions follow the security
rules, and group-scoped authorization prevents access outside assigned groups.

### 4.2 Configuration governance

1. Open the test group.
2. Create and validate a new configuration version.
3. Submit it for approval with a different eligible reviewer.
4. Approve it and deploy it to the snapshotted targets.
5. Confirm deployment history, desired configuration and effective
   configuration.
6. Introduce a controlled test difference and verify drift visibility.

Expected result: the immutable version, approval, target snapshot, deployment,
effective state, drift and audit entries refer to the same governed change.

### 4.3 Governed Collector membership

Use at least three test Collectors and two enabled groups.

1. Open **Groups & Labels** and create a group.
2. Select two currently unassigned Collectors during creation.
3. Confirm a Collector already assigned elsewhere is not offered.
4. Create a second group with a controlled wildcard or regex rule.
5. Reconnect an otherwise unassigned Collector whose reported identity matches.
6. Confirm it automatically joins the expected group.
7. Explicitly remove it and confirm it remains excluded after reconnecting.
8. Open **Manage Collector membership** on the first group.
9. Create an assignment proposal that keeps the current configuration.
10. Create a move proposal that selects an approved destination-group
    configuration.
11. Confirm the impact preview shows the operation, target count, destination
    and configuration policy.
12. Approve each proposal as the assigned reviewer.
13. Expand **View execution evidence** in Membership History.

Expected result: only authorized and currently eligible Collectors can be
selected; destination configurations belong to the destination group; completed
targets contain configuration and completion evidence.

#### Controlled batch failure and recovery

1. Create a proposal containing multiple disposable test Collectors.
2. Introduce a controlled failure for a later target after at least one earlier
   target can complete.
3. Approve and execute the request.
4. Inspect the request error and each Collector result.
5. Verify earlier changed targets report `restored` when compensation succeeds.
6. Verify the failing target reports `failed` or `rollback_failed`.
7. Verify targets not reached by execution report `not_started`.
8. Confirm original membership and previous configurations were restored where
   evidence reports `restored`.

Expected result: FleetAMP does not hide partial execution. It attempts
compensating rollback in reverse order and records any incomplete restoration
for operator action.

## 5. Migration workflow

1. Open **Migration** and paste or upload valid Collector YAML.
2. Parse and normalize the configuration.
3. Review standardization findings and pattern matches.
4. Run the **Validate** stage.
5. Review the final preview before saving.
6. Save the result as an immutable group configuration version.
7. Confirm it appears in Migration History.
8. Optionally submit the exact saved version for approval.

Expected result: invalid input cannot be saved; preview content matches the
saved immutable version; history and optional approval preserve its identity.

## 6. Guides, Blueprints and secrets

1. Follow one path through **Guides** and confirm only compatible choices are
   progressively displayed.
2. Select a Blueprint, destination profile and accessible group.
3. Generate and validate the Collector configuration.
4. If the configuration needs a credential, create a group secret and use a
   `${secret:key}` reference.
5. Confirm the raw secret does not appear in previews, versions, approval diffs
   or audit details.

Expected result: educational guidance remains read-only; generated
configuration enters the normal immutable validation and approval workflow;
secret values remain write-only.

## 7. Component lifecycle and GitOps

### 7.1 Repository connection

1. Open **Administration → Integrations**.
2. Enter a name and select GitHub, GitLab or Azure DevOps.
3. Confirm only fields relevant to the selected provider are displayed.
4. Limit the connection to the test group and a safe allowed root.
5. Save the connection using a `secret://` credential reference.
6. Run connection validation.

Expected result: immutable validation evidence records repository and branch
access without exposing the credential. Failed, missing or expired validation
prevents execution queueing.

### 7.2 Proposal, target resolution and approval

1. Open **Deployments** and create an install, upgrade, restart or removal
   proposal.
2. Select the component type, test group and compatible deployment method.
3. Resolve targets and review every compatibility finding.
4. Submit the immutable proposal for approval.
5. Sign in as the assigned reviewer and approve the exact proposal evidence.

Expected result: resolved targets are group-scoped; blocking compatibility
findings prevent preparation; the requester cannot approve their own proposal.

### 7.3 Immutable preview and execution

1. Select the enabled and currently validated Git connection.
2. Generate the GitOps preview.
3. Verify the repository path remains below the configured allowed root.
4. Review the files and diff, then submit the exact preview for approval.
5. Approve and queue it in the execution outbox.
6. Open **GitOps execution monitor** and follow its append-only timeline.

Expected result: preview, approval and execution share the same preview and plan
hashes. Dry-run mode records success without modifying a repository.

### 7.4 Optional GitHub pull-request delivery

Enable real writes only for a dedicated test repository and least-privilege
token. GitLab and Azure DevOps remain dry-run delivery providers in v0.3.0.

1. Enable the GitOps worker and GitHub writes.
2. Queue an approved GitHub pull-request execution.
3. Confirm FleetAMP creates or reuses the deterministic FleetAMP branch and one
   pull request containing exactly the approved files.
4. Merge or close the pull request.
5. Confirm status synchronization appends the corresponding change event.

Expected result: retries and worker recovery reuse the execution ID and branch;
they do not create duplicate pull requests.

## 8. Statistics and audit

1. Confirm Integration statistics classify connections as valid, failed,
   expired or untested.
2. Confirm the GitOps monitor classifies in-progress, open, completed,
   failed/closed and retried executions.
3. Filter Audit Log by **GitOps & Integrations**.
4. Trace one test change from connection validation through preview, approval,
   queueing and completion.

Expected result: totals match visible immutable records and the audit entries
identify actor, action, resource and outcome without credentials.

## 9. Failure and recovery checks

| Scenario | Expected behavior |
| --- | --- |
| Invalid Collector YAML | Validation fails and no version is saved |
| Collector already belongs to an enabled group | Assignment proposal is rejected as stale |
| Destination configuration belongs to another group | Membership proposal is rejected |
| Later target fails in a membership batch | Earlier targets are restored or expose rollback failure evidence |
| Dynamic rule overlaps an enabled group | Group creation or update is rejected |
| Disabled or unauthorized connection | Preview preparation fails |
| Missing, failed or expired connection validation | Execution cannot be queued |
| Repository path traversal attempt | Path resolution rejects it |
| Worker lease expires | Another worker may reclaim the same execution |
| Provider execution fails | Failure evidence is appended |
| Administrator retries with a reason | Same execution and deterministic branch are reused |
| Retry limit reached | Further retry is rejected |

## 10. Release decision

The release candidate is ready only when:

- the full automated verifier passes;
- the copied v0.2 database starts, migrates and restarts successfully;
- all applicable E2E scenarios pass;
- no credentials appear in logs, audit data, previews or saved versions;
- release-blocking defects are closed or explicitly documented;
- known limitations in the release notes match the tested behavior.

Attach the completed checklist, relevant screenshots and sanitized logs to the
release PR. Do not attach tokens, passwords, private keys or raw secret values.
