# Governed Collector membership

FleetAMP v0.3.0 manages which OpenTelemetry Collectors belong to an ownership
group. Membership changes are governed operations: FleetAMP snapshots the
selected Collectors, requires a reviewer, records the execution result for each
Collector and attempts compensating rollback when a batch does not complete.

## What this workflow controls

Collector membership determines:

- which group owns and displays the Collector;
- which Group Owners may view and operate it;
- which group configuration can be selected during assignment or movement;
- whether an explicitly removed Collector may automatically rejoin a dynamic
  group; and
- which immutable approval and audit records explain the change.

Changing membership is not the same as creating a configuration. A membership
proposal may keep the Collector's current configuration or select an approved
configuration version belonging to the destination group.

## Roles and permissions

| Role | Visibility and actions |
| --- | --- |
| Administrator | Create and manage all groups, membership rules and proposals |
| Group Owner | Manage membership only inside authorized groups |
| Operator | Work within permissions granted by the active role policy |
| Reviewer | Approve only requests assigned to that user |
| Viewer | Read authorized fleet and group state; cannot mutate membership |

FleetAMP applies server-side authorization even when a navigation item or
button is hidden. The requester and assigned reviewer remain separate actors.

## Membership models

### Explicit membership

During group creation, users can select from Collectors that are currently
unassigned. A Collector already owned by another enabled group is not offered
as an available target.

After creation, open the group and select **Manage Collector membership** to
assign more unassigned Collectors, remove members or move members to another
group.

### Dynamic membership

A group may define one controlled wildcard or regular-expression rule for
Collectors that are rebuilt frequently or have short lifetimes. Supported
reported identities are:

- stable Collector ID;
- Collector name;
- FleetAMP display name;
- reported `host.name`; or
- another reported attribute with an explicit attribute key.

Example wildcard:

```text
payments-prod-collector-*
```

FleetAMP validates the rule and rejects overlap with an existing enabled group.
A Collector explicitly removed from a dynamic group is added to that
Collector's exclusion list so it does not immediately rejoin through the same
rule.

## Create a group and assign Collectors

1. Open **Groups & Labels**.
2. Select **Create group**.
3. Enter the group name, description and ownership labels.
4. Select only the available unassigned Collectors that belong to the intended
   team or service.
5. Optionally enable dynamic membership and define the reported identity,
   wildcard or regex method, and pattern.
6. Review the matching preview.
7. Create the group.

The new group page shows its members, active Collectors, configuration versions,
approval requests and drift count.
## Create a governed membership proposal

Open **Groups & Labels**, select the group name, and open **Manage Collector
membership**. The page provides three operations.

### Assign available Collectors

1. Select one or more unassigned Collectors.
2. Choose **Keep current configuration** or an approved destination-group
   configuration.
3. Review the impact preview.
4. Select an eligible reviewer and enter a reason.
5. Submit the assignment request.

### Remove Collectors

1. Select one or more current members.
2. Choose one of the configuration policies:
   - **Keep current configuration** leaves the most recently assigned
     configuration in place.
   - **Restore pre-group configuration** requests the configuration captured
     before the Collector entered the group.
3. Review the impact preview.
4. Select a reviewer, enter a reason and submit.

Removing a Collector from a group does not delete the Collector from FleetAMP.

### Move Collectors

1. Select one or more members of the current group.
2. Select an enabled destination group.
3. Choose **Keep current configuration** or an approved configuration belonging
   to the destination group.
4. Review the resolved destination and affected Collectors.
5. Select a reviewer, enter a reason and submit.

FleetAMP validates the group and configuration relationship again on the
server. A stale or unauthorized destination configuration is rejected even if
a client submits it directly.

## Approval and execution

A proposal stores immutable target snapshots containing the Collector identity,
source membership and previous configuration reference. At approval time,
FleetAMP validates every target before modifying any Collector.

The assigned reviewer can:

- approve the exact membership proposal; or
- reject it with a review comment.

For an approved batch, FleetAMP:

1. resolves every snapshotted Collector;
2. confirms current membership still matches the approved source;
3. captures the current group fields, exclusions and configuration assignment;
4. delivers the selected configuration when applicable;
5. changes membership; and
6. saves the durable Collector snapshot.

If validation fails before execution, no Collector is changed.

## Failure handling and compensating rollback

Membership delivery spans FleetAMP state and remote OpAMP configuration
delivery, so it is not a distributed ACID transaction. FleetAMP provides
compensating transactional behavior.

If a later Collector fails after earlier Collectors changed, FleetAMP processes
the completed checkpoints in reverse order and attempts to:

- restore original group fields;
- restore dynamic-group exclusions;
- deliver the previous configuration; and
- persist the restored inventory snapshot.

When no previous configuration was recorded, FleetAMP cannot safely invent one.
That condition is reported as an incomplete rollback and requires operator
review.

## Per-Collector execution evidence

Expand **View execution evidence** in Membership History to inspect each
Collector. Possible final states are:

| State | Meaning |
| --- | --- |
| `completed` | Membership and applicable configuration actions completed |
| `failed` | This Collector caused or encountered an execution failure |
| `restored` | The Collector changed, then compensating rollback succeeded |
| `rollback_failed` | One or more restoration steps failed |
| `not_started` | Execution stopped before this Collector was processed |

Each result can include the selected configuration ID, restored configuration
ID, completion timestamp and a Collector-specific error. The parent request
also retains its requester, reviewer, reason, decision, overall status and
error.
## Verification checklist

Use a disposable lab and at least three Collectors.

1. Create a group and select two unassigned Collectors.
2. Confirm an already assigned Collector is unavailable in another group's
   creation form.
3. Create a wildcard rule and reconnect a matching test Collector.
4. Confirm the new Collector joins the expected group.
5. Explicitly remove the dynamically matched Collector and confirm it remains
   excluded.
6. Assign a Collector while keeping its current configuration.
7. Move a Collector using an approved destination configuration.
8. Remove a Collector and restore its pre-group configuration.
9. Approve a multi-Collector request and inspect execution evidence.
10. Exercise a controlled failure and confirm earlier targets show
    `restored` or `rollback_failed`, while unprocessed targets show
    `not_started`.
11. Confirm the Audit Log identifies the actor, operation and outcome.
12. Sign in as a Group Owner and confirm unrelated groups and Collectors remain
    inaccessible.

Use the complete [v0.3.0 E2E checklist](e2e-v0.3.0.md) before release.

## Operational boundaries

- Only approved configurations belonging to the destination group are offered.
- Membership rules match reported identity; they do not establish trust by
  themselves. Secure OpAMP authentication remains required.
- Dynamic matching is evaluated for otherwise unassigned Collectors.
- Explicit removal prevents immediate re-entry into the same dynamic group.
- Configuration delivery is asynchronous over OpAMP.
- Rollback is compensating and best effort; incomplete restoration is visible
  in per-Collector evidence and the request error.
- Kubernetes API, systemd and container lifecycle execution are separate
  lifecycle features and are not performed by a membership move.
