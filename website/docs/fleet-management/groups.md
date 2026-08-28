---
title: Labels, Groups and Selectors
---

# Labels, Groups and Selectors

FleetAMP Groups v1 provides dynamic fleet targeting without persisting static membership lists. Group selectors can match metadata reported by the Collector through OpAMP as well as FleetAMP-managed labels. FleetAMP labels take precedence when both sources provide the same key.

## Labels

Labels are management metadata owned by FleetAMP rather than metadata reported by OpAMP. Examples include `team`, `environment`, `region`, and `role`. FleetAMP preserves these labels across OpAMP heartbeats and reconnects.

Replace an agent's label set:

```bash
curl -X PUT http://localhost:8080/api/v1/agents/<INSTANCE_UID>/labels \
  -H 'Content-Type: application/json' \
  -d '{"team":"payments","environment":"prod","region":"eu-west-1"}'
```

Use `PATCH` to merge labels. Sending an empty value with `PATCH` removes that label.

## Groups

A group stores a name, description, and selector. Membership is computed dynamically from effective targeting metadata: reported OTel/OpAMP attributes first, with FleetAMP-managed labels overlaid as operator overrides.

```json
{
  "name": "payments-prod",
  "description": "Payments production collectors",
  "selector": {
    "team": "payments",
    "environment": "prod"
  }
}
```

All selector entries use AND semantics. An agent must match every selector label. Empty selectors are rejected so a group cannot accidentally target the entire fleet.

## Collector-provided targeting metadata

A Collector can provide targeting values through its OpAMP agent description. This lets a packaged Collector identify its team, environment or application without requiring an operator to re-enter the same values in FleetAMP.

```yaml
agent_description:
  non_identifying_attributes:
    team: payments
    environment: prod
    application: payment-api
```

If a FleetAMP group has the selector `team=payments,environment=prod`, the Collector matches that group automatically after FleetAMP receives the attributes. The group must already exist in FleetAMP; a Collector does not create or redefine groups.

If FleetAMP also has a managed label with the same key, the managed label wins. For example, a reported `environment=prod` combined with a FleetAMP label `environment=dev` targets the Collector as `environment=dev`. This provides a deliberate operator override while keeping both sources visible on Agent Details.

## API

Create and list groups:

```bash
curl -X POST http://localhost:8080/api/v1/groups \
  -H 'Content-Type: application/json' \
  -d '{"name":"payments-prod","selector":{"team":"payments","environment":"prod"}}'

curl http://localhost:8080/api/v1/groups
```

Inspect a group and its current members:

```bash
curl http://localhost:8080/api/v1/groups/<GROUP_ID>
curl http://localhost:8080/api/v1/groups/<GROUP_ID>/members
```

`PUT /api/v1/groups/<GROUP_ID>` updates the group definition and `DELETE /api/v1/groups/<GROUP_ID>` removes it.

## UI

The main `/agents` page shows each agent's matching group names and includes a **Group** dropdown next to the time-range filter. Selecting a group filters the fleet inventory to agents that currently match that group's selector.

The `/groups` page can create groups, show selectors and current member counts, and open a group details page containing the matching agents. Agent Details shows the agent's FleetAMP labels and every group it currently matches.

Agent Details also provides an **Add to group** dropdown. Choosing a group merges that group's selector labels into the agent's existing FleetAMP labels. For example, selecting a group with `team=payments,environment=prod` applies those two labels to the collector, causing it to match the group immediately. Unrelated labels are preserved.

This action does not create static membership. Group membership still changes automatically when labels change, and FleetAMP does not write membership rows into the database.

## Scope of Groups v1

Groups v1 establishes reliable fleet targeting only. Configuration deployment to a group, staged rollout, rollout thresholds, and group rollback are separate milestones built on top of this selector model.

## Add an existing running Collector to a group

You do not need to restart, redeploy, or change the OpenTelemetry Collector configuration. First create the target group from **Groups → Create group**. Then open **Agents**, select the already-running Collector, find the **Groups** card, choose the target from **Add to group**, and click **Apply group labels**.

FleetAMP assigns the group's controlled identity fields to the managed-agent record and recalculates membership immediately. For a `payments-prod` selector of `team=payments,environment=prod`, those labels are attached to the managed-agent record; they are not injected into `otelcol` YAML or sent as telemetry resource attributes.

If the **Add to group** dropdown is empty, create at least one group first. To move an agent out of a selector-based group, edit/remove the FleetAMP labels that make it match; static membership is intentionally not stored.

## Metadata sources and ownership

FleetAMP should support multiple metadata sources, but it should not silently collapse them into one unowned map. The intended model is:

```text
OTel / OpAMP reported attributes  -> observed metadata (read-only)
FleetAMP UI/API labels            -> managed metadata (FleetAMP-owned)
CMDB / ServiceNow enrichment      -> external metadata (provider-owned)
                                      |
                                      v
                             targeting / selectors
                                      |
                                      v
                                   groups
```

This preserves provenance and makes conflicts understandable. For example, a Collector may report `deployment.environment=prod` while a FleetAMP administrator adds `team=payments`, and ServiceNow may later enrich it with `business_service=payments-api`. FleetAMP should retain the source of each value rather than overwrite one source with another.

The Agent Details UI therefore shows **FleetAMP labels** separately from **Reported OTel attributes**. FleetAMP labels can be added individually with **+ Add label**. Reported attributes are read-only because their source of truth is the Collector/OpAMP side.

### Targeting model

Groups match **effective targeting metadata**. Reported OTel/OpAMP attributes provide the base values and FleetAMP-managed labels override the same key when an operator needs to correct or supplement targeting. Source-aware external enrichment remains a future extension.

Direct static membership is intentionally avoided. Dynamic selectors are easier to audit and scale because membership can always be explained by the metadata that caused the match.

## Creating a group in the UI

FleetAMP groups now use exactly three mandatory identity fields: **Application**, **Place**, and **Environment**. The group creation page presents each as a separate user-input field; there is no additional-selector control on the group form.

These three values define group membership and are intended to become the stable target identity for access policy and configuration deployment. A Collector can report the same keys through OpAMP `agent_description.non_identifying_attributes`, or FleetAMP-managed labels can provide/override them.

Flexible metadata such as `team`, `owner`, `contact`, `business_service`, or other ServiceNow-enriched values belongs in the **Labels** section, where customers can add arbitrary key/value metadata without changing group identity.

Example group identity:

```text
Application = payment-api
Place       = eu-west-1
Environment = prod
```

A Collector reporting the same values automatically matches the group.

## Controlled group identity

FleetAMP intentionally keeps group creation constrained. Customers enter only three required identity fields: `Application`, `Environment`, and `Place`. FleetAMP derives the group name automatically as `<application>-<environment>-<place>` after normalizing the values to lowercase URL-safe text.

For example, `payment-api`, `prod`, and `eu-west-1` produces the group name `payment-api-prod-eu-west-1`. Users do not type an arbitrary group name and cannot add extra group fields from the group form. This keeps group identity predictable for membership, deployment targeting, and future access-control scoping.

Flexible business and ownership metadata belongs in **Labels**, not in the group definition. Labels can later be governed through FleetAMP, ServiceNow enrichment, or other integrations, for example `team=payments`, `owner=platform-team`, or `contact=payments-oncall`.


## Controlled group identity and optional labels

FleetAMP deliberately separates controlled group identity from open-ended labels. Group identity supports exactly three keys: `application`, `environment`, and `place`. These fields are used for group membership and are intended to become the stable scope for deployment and access control. Labels are optional metadata such as `team`, `owner`, `contact`, `support_group`, or `business_service`.

### Collector to FleetAMP

The preferred OpAMP `AgentDescription.non_identifying_attributes` namespace is:

```yaml
agent_description:
  non_identifying_attributes:
    fleetamp.group.application: payment-api
    fleetamp.group.environment: prod
    fleetamp.group.place: eu-west-1
    fleetamp.label.team: payments
    fleetamp.label.owner: payments-platform
    fleetamp.label.contact: payments-oncall
```

After the Collector or Supervisor reconnects, FleetAMP displays the three group fields under **Group identity** and the `fleetamp.label.*` values under **Labels**. FleetAMP also retains the raw AgentDescription attributes for troubleshooting.

Only `fleetamp.group.application`, `fleetamp.group.environment`, and `fleetamp.group.place` are valid group keys. If a Collector reports a key such as `fleetamp.group.aplication`, FleetAMP records it as an **Unknown group field** on Agent Details. `fleetamp.label.*` remains intentionally open-ended.

For compatibility with early FleetAMP experiments, raw `application`, `environment`, and `place` attributes are still recognized, but the `fleetamp.group.*` namespace is recommended because it allows reliable validation.

### FleetAMP UI to Collector

A group selected in FleetAMP becomes managed group identity, and labels added in FleetAMP become managed labels. These values persist in FleetAMP across Collector reconnects. FleetAMP-managed values override the same values reported by the Collector when calculating effective state.

OpAMP `AgentDescription` itself is agent-to-server data; the server does not directly mutate it. To make FleetAMP-managed group identity and labels appear inside the Collector, FleetAMP must deliver an equivalent Collector configuration through the normal OpAMP remote-config path. This outbound metadata synchronization is a distinct deployment action and should use the same validation, delivery status, rollback, and audit mechanisms as other FleetAMP configuration deployments.

### Group administration

The Groups UI supports create, edit/update, and delete. Group names are generated from the controlled identity as `<application>-<environment>-<place>` and cannot be arbitrarily supplied in the UI. Updating any of the three fields regenerates the group name.

### Label governance

FleetAMP currently allows a maximum of **5 FleetAMP-managed labels per agent**. Adding a sixth distinct managed label through the UI or REST API is rejected. Updating the value of an existing label is still allowed when the agent already has five labels. This limit controls operator/integration-created metadata while the label key space remains flexible.

The existing **Assign group** action on Agent Details remains supported. It assigns only the controlled group identity (`application`, `environment`, `place`); it does not consume any of the five optional managed-label slots.
