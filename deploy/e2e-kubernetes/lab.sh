#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
NAMESPACE="fleetamp-e2e"
ACTION="${1:-help}"

build_images() {
  docker build --build-arg VERSION=e2e -t fleetamp:e2e "$ROOT_DIR"
  docker build -t fleetamp-opamp-supervisor:0.149.0 "$ROOT_DIR/deploy/lab/opamp-supervisor"
}

apply_lab() {
  kubectl apply -k "$ROOT_DIR/deploy/e2e-kubernetes"
  kubectl rollout status deployment/fleetamp -n "$NAMESPACE" --timeout=180s
  kubectl rollout status deployment/collector-new -n "$NAMESPACE" --timeout=180s
  kubectl rollout status deployment/collector-fresh -n "$NAMESPACE" --timeout=180s
  kubectl rollout status deployment/collector-existing -n "$NAMESPACE" --timeout=180s
  kubectl get pods -n "$NAMESPACE" -o wide
}

status_lab() {
  kubectl get deployment,pod,service,pvc -n "$NAMESPACE" -o wide
}

logs_lab() {
  kubectl logs -n "$NAMESPACE" deployment/fleetamp --tail=100
  kubectl logs -n "$NAMESPACE" deployment/collector-new --tail=50
  kubectl logs -n "$NAMESPACE" deployment/collector-fresh --tail=50
  kubectl logs -n "$NAMESPACE" deployment/collector-existing --tail=50
}

port_forward() {
  echo "FleetAMP E2E will be available at http://localhost:18081"
  echo "Your systemd FleetAMP remains at http://localhost:8080"
  kubectl port-forward -n "$NAMESPACE" service/fleetamp 18081:8080
}

reset_collectors() {
  kubectl rollout restart deployment/collector-new deployment/collector-fresh deployment/collector-existing -n "$NAMESPACE"
  kubectl rollout status deployment/collector-new -n "$NAMESPACE" --timeout=180s
  kubectl rollout status deployment/collector-fresh -n "$NAMESPACE" --timeout=180s
  kubectl rollout status deployment/collector-existing -n "$NAMESPACE" --timeout=180s
}

delete_lab() {
  kubectl delete namespace "$NAMESPACE" --wait=true
}

case "$ACTION" in
  build) build_images ;;
  apply) apply_lab ;;
  up) build_images; apply_lab ;;
  status) status_lab ;;
  logs) logs_lab ;;
  port-forward) port_forward ;;
  reset-collectors) reset_collectors ;;
  delete) delete_lab ;;
  *)
    echo "Usage: bash deploy/e2e-kubernetes/lab.sh {build|apply|up|status|logs|port-forward|reset-collectors|delete}"
    exit 1
    ;;
esac
