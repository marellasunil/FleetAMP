# FleetAMP Kubernetes E2E Lab

This lab deploys a self-contained FleetAMP environment into the `fleetamp-e2e` namespace. It does not modify the host systemd service on `localhost:8080` or workloads in the `observability` namespace.

## Scenarios

| Workload | Starting state | Test purpose |
| --- | --- | --- |
| `collector-new` | Supervisor connected without an initial Collector configuration | First enrollment and first governed deployment |
| `collector-fresh` | Supervisor with a minimal OTLP traces pipeline | Groups, labels, approval, deployment, rollback and drift |
| `collector-existing` | Standalone OTel Collector with OpAMP and an existing metrics/traces configuration | Migration, standardization, pattern matching and adoption assessment |

The existing collector reports effective configuration through the OpAMP extension. It does not claim the Supervisor capability to accept remote configuration. This distinction keeps the adoption test technically accurate.

## Requirements

- Docker
- Docker Desktop Kubernetes or another local Kubernetes cluster
- `kubectl`
- A default StorageClass
- Local images available to the Kubernetes runtime

## Start

```bash
cd ~/Desktop/Sunil/MyGitRepo/FleetAMP
bash deploy/e2e-kubernetes/lab.sh up
```

If Kubernetes reports `ErrImageNeverPull`, load the two local images into the cluster runtime. Docker Desktop normally shares local images. For kind, run:

```bash
kind load docker-image fleetamp:e2e
kind load docker-image fleetamp-opamp-supervisor:0.149.0
bash deploy/e2e-kubernetes/lab.sh apply
```

## Access without changing localhost:8080

Run in a separate terminal:

```bash
bash deploy/e2e-kubernetes/lab.sh port-forward
```

Open <http://localhost:18081>. The systemd installation remains available at <http://localhost:8080>.

## Operations

```bash
bash deploy/e2e-kubernetes/lab.sh status
bash deploy/e2e-kubernetes/lab.sh logs
bash deploy/e2e-kubernetes/lab.sh reset-collectors
```

## Clean reset

Restarting collectors preserves the FleetAMP database:

```bash
bash deploy/e2e-kubernetes/lab.sh reset-collectors
```

Deleting the namespace removes the lab, including its PVC and test database:

```bash
bash deploy/e2e-kubernetes/lab.sh delete
```

Use `delete` only when the E2E evidence and any required screenshots have been saved.

## Release test sequence

1. v0.1.0: setup, authentication, agent inventory and first deployment.
2. v0.2.0: group assignment, labels, validation, approval, deployment, effective configuration, drift and rollback.
3. v0.3.0: import the existing collector configuration, apply the safe baseline, review adoption readiness, match a Pattern, preview adoption changes and review fleet-wide adoption.
4. Record unimplemented Migration stages as limitations rather than successful tests.
