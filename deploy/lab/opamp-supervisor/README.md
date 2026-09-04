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

## Preserve identity across pod restarts

The Supervisor writes its OpAMP instance identity under `storage.directory`. Kubernetes must persist that path; an `emptyDir` creates a new FleetAMP agent whenever the pod is recreated.

```bash
kubectl apply -f supervisor-storage-pvc.yaml
kubectl patch deployment otel-gateway --namespace observability \
  --type json --patch-file supervisor-storage-patch.yaml
```

The deployment must mount the `supervisor-storage` volume at `/var/lib/otel-supervisor`. Give every logical Collector a unique, stable `fleetamp.agent.id` under `agent.description.non_identifying_attributes`, as shown in `supervisor.yaml`. FleetAMP uses that explicit ID as a guarded fallback to carry managed group identity and labels to a replacement InstanceUID. It does not guess identity from pod names or generic service names.

For multiple gateway replicas, use a StatefulSet and one PVC plus one unique stable ID per replica. Do not share a Supervisor storage directory or `fleetamp.agent.id` between simultaneously running replicas.

The verified remote-config lifecycle was:

```text
sent -> applying -> applied
```

FleetAMP then displayed both desired and effective configuration on the Agent Details page.
