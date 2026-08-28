---
title: Troubleshooting
---
# Troubleshooting

If a Collector does not appear in **Agents**, confirm that FleetAMP is listening on the OpAMP endpoint, that the Collector/Supervisor can reach it, and that the endpoint path is `/v1/opamp`.

If remote configuration is `unsupported`, inspect the agent capabilities. Direct Collector OpAMP connectivity can report state, but remote configuration requires an agent that advertises `accepts_remote_config`; FleetAMP's verified remote-management path uses the OpAMP Supervisor.

If a group does not contain an expected Collector, compare the group's selector with the Collector's FleetAMP labels. All current selector entries use AND semantics.
