package lifecycle

import (
	"errors"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

func TestNewRequestValidatesOperationsAndProducesStableSpecHash(t *testing.T) {
	tests := []Spec{
		{Operation: Install, ComponentType: runtimes.OTelCollector, GroupID: "payments", DeploymentMethod: "package", DesiredVersion: "0.149.0", Reason: "new host agents"},
		{Operation: Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "approved security update"},
		{Operation: Restart, ComponentType: runtimes.OTelCollector, GroupID: "payments", DeploymentMethod: "systemd", CurrentVersion: "0.149.0", Reason: "recover unhealthy process"},
		{Operation: Remove, ComponentType: runtimes.OTelOperator, GroupID: "sandbox", DeploymentMethod: "gitops", CurrentVersion: "0.149.0", Reason: "retire sandbox"},
	}
	for _, spec := range tests {
		first, err := NewRequest(spec, "operator")
		if err != nil {
			t.Fatalf("NewRequest(%s): %v", spec.Operation, err)
		}
		second, err := NewRequest(spec, "operator")
		if err != nil {
			t.Fatal(err)
		}
		if first.ID == second.ID || first.SpecHash != second.SpecHash || len(first.SpecHash) != 64 || first.Status != Proposed {
			t.Fatalf("unexpected immutable identity/hash for %s", spec.Operation)
		}
	}
}

func TestNewRequestRejectsInvalidVersionSemantics(t *testing.T) {
	_, err := NewRequest(Spec{Operation: Upgrade, ComponentType: runtimes.OTelCollector, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.149.0", DesiredVersion: "0.149.0", Reason: "no change"}, "operator")
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("NewRequest() error = %v, want ErrInvalidSpec", err)
	}
}
