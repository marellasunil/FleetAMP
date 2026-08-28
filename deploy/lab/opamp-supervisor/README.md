# Kubernetes OpAMP Supervisor lab

This lab runs the OpenTelemetry OpAMP Supervisor in front of an OTel Collector Gateway and connects the Supervisor to FleetAMP.

Verified with Supervisor and Collector `0.149.0`.

## Flow

```text
FleetAMP :4320
    |
    | OpAMP remote config
    v
OpAMP Supervisor
    |
    | manages process + effective config
    v
OTel Collector Gateway :4317/:4318
```

The Supervisor automatically injects the Collector-side OpAMP extension. Do not keep a direct FleetAMP OpAMP extension in the local base config.

Use `supervisor.yaml` as the Supervisor configuration and create the Collector base ConfigMap from your own sanitized/local Collector config. Do not commit backend credentials or tokens into this directory.

The verified remote-config lifecycle was:

```text
sent -> applying -> applied
```

FleetAMP then displayed both desired and effective configuration on the Agent Details page.
