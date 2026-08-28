---
title: Metadata and Labels
---
# Metadata and Labels

FleetAMP keeps metadata ownership explicit. **Reported OTel attributes** come from the Collector through OpAMP and are read-only in FleetAMP. **FleetAMP labels** are management metadata owned by FleetAMP and can be added from the Collector details page with **+ Add label** or through the labels API.

Examples of managed labels are `team=payments`, `environment=prod`, `region=eu-west-1`, and `role=gateway`. These labels survive Collector heartbeats and reconnects.

Groups use effective targeting metadata built from both sources. Reported OTel/OpAMP attributes provide the base values, and FleetAMP-managed labels override matching keys. Keeping the sources separate in the Agent Details UI preserves ownership while allowing either source to drive group membership.

## Targeting fields

`application` and `environment` are FleetAMP's mandatory group-targeting keys. Customers can report them from the Collector through OpAMP `agent_description.non_identifying_attributes`, or FleetAMP operators can add them as managed labels. Optional keys such as `team`, `region`, and `role` can be added in the same way.

FleetAMP preserves the two sources separately. For targeting, reported attributes form the base and a FleetAMP-managed label with the same key overrides the reported value. The resulting effective targeting metadata is shown on Agent Details and is used to calculate group membership.

## Group identity versus labels

`application`, `place`, and `environment` are the mandatory identity dimensions used by FleetAMP groups. Labels remain the extensible metadata layer for operational ownership and enrichment, for example `team=payments`, `owner=platform-observability`, or `contact=payments-oncall`. Future ServiceNow integration should enrich labels rather than redefine the core group identity.
