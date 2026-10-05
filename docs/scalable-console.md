# Scalable console navigation

FleetAMP uses task-focused tabs, filters, and bounded result pages so the same
console remains usable for a small team or an organization with thousands of
Collectors and ownership groups.

## Fleet

The Fleet page contains these views:

- **Agents** — live Collector inventory, health, group name, version, current
  configuration, lifecycle filtering, and 25/50/100-row pagination.
- **Pipelines** — planned signal flow inventory derived from deployed
  configurations.
- **AI Insights** — a governed preview for explainable risk and operational
  recommendations.
- **SLOs & Alerts** — a preview of fleet availability, deployment, drift, and
  approval objectives.

Preview tabs describe planned capabilities and do not display synthetic data as
if it were live.

## Groups & Labels

- **Groups** contains ownership boundaries and their human-readable names.
- **Labels** aggregates selector keys and values across groups.
- **Selectors** provides a compact rule catalogue for reviewing targeting at
  scale.

FleetAMP displays group names in the interface while retaining stable group IDs
for APIs, links, authorization, and storage. Group and selector lists use
25/50/100-row pagination.
## Users, access groups, and roles

Administration separates identity concepts into three tabs:

- **Users** lists accounts, status, role, access scope, and last login.
- **Access Groups** summarizes membership against existing Collector ownership
  groups. This is a compatibility view, not a second group database.
- **Roles** documents built-in administrator, group-owner, and viewer
  permissions.

Fine-grained custom roles and identity-provider synchronization remain planned.
Until that schema is introduced, Collector ownership groups are the source of
truth for a user's group scope.

## Audit Log

Audit events can be reviewed as All Activity, Configuration, Deployment, Drift,
or Security & Users. Date, actor, search, and event-type filters are preserved
when moving between pages. Results use the shared 25/50/100-row pagination.

## Navigation principles

The sidebar contains stable product areas; closely related views live in tabs.
Pipelines and AI Insights therefore live under Fleet rather than appearing as
duplicate sidebar destinations. Upcoming features are visibly marked Preview or
Upcoming and existing authorization checks continue to apply to every route.

## Group-scoped RBAC

FleetAMP separates organization-wide administration from group responsibilities.
Platform Admin is the only global role. All other permissions are attached to a
user's membership in a specific Collector Group.

Built-in group roles are Group Owner, Configuration Editor, Deployment
Approver, Deployment Operator, Auditor, and Viewer. A member may hold several
roles in one group and different roles in another group. FleetAMP preserves the
legacy role and group fields during the compatibility period, while normalized
membership tables are the source of truth for new assignments.

The Users tab is an identity summary. Group membership and role assignment are
managed from Group Members. Administrators reauthenticate before resetting
another user's password; the temporary password forces a change at next
sign-in. Users verify their current password when changing it from My Account.
Short-lived single-use recovery tokens provide a host-controlled break-glass
path when no administrator can sign in.
