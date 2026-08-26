# FleetAMP Deployment Model

FleetAMP manages telemetry agents independently of where they run.

Supported deployment contexts are modeled separately from the telemetry agent type.

Examples:

- OpenTelemetry Collector on an EC2 instance
- OpenTelemetry Collector on a Linux VM
- OpenTelemetry Collector as a Kubernetes DaemonSet
- OpenTelemetry Collector as a Kubernetes Deployment/Gateway
- Grafana Alloy on a VM
- Grafana Alloy in Kubernetes

## Separation of responsibilities

FleetAMP is responsible for the telemetry-agent control plane:

- inventory
- health and connection state
- configuration targeting
- remote configuration
- desired-vs-effective state
- grouping and labels
- rollout status

The underlying runtime remains responsible for workload lifecycle:

- Kubernetes manages Pods, Deployments, DaemonSets, scheduling, replicas, and restarts
- cloud/VM tooling manages VM/instance lifecycle
- systemd or other local service managers manage host processes when appropriate

FleetAMP should not become a replacement for Kubernetes, Terraform, cloud autoscaling, or operating-system service management.

## Runtime metadata

The generic `ManagedAgent` model contains a `DeploymentContext` with normalized runtime metadata such as:

```json
{
  "runtime": "kubernetes",
  "provider": "aws",
  "cluster": "prod-eu",
  "namespace": "observability",
  "node": "worker-07"
}
```

Detailed reported metadata remains available through `Attributes`, for example:

```text
k8s.cluster.name
k8s.namespace.name
k8s.pod.name
k8s.node.name
cloud.provider
cloud.region
host.name
```

FleetAMP-owned `Labels` remain separate and can be used for targeting:

```text
team=payments
environment=prod
role=agent
runtime=kubernetes
```

## Kubernetes example

```text
Kubernetes
  DaemonSet / Deployment
          |
          v
  telemetry agent
          |
          | management protocol
          v
      FleetAMP
```

Kubernetes answers: "How many instances should run and where?"

FleetAMP answers: "What configuration and management policy should those agents receive?"

This separation lets FleetAMP manage the same logical fleet across Kubernetes, VMs, containers, and bare metal without coupling the core project to one deployment platform.
