package lifecycle

import (
	"testing"

	"github.com/marellasunil/FleetAMP/internal/runtimes"
)

func TestNewApprovalPinsCompatibleEvidence(t *testing.T) {
	request, err := NewRequest(Spec{Operation: Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security update"}, "operator")
	if err != nil { t.Fatal(err) }
	validation := &Validation{ID: "validation-1", RequestID: request.ID, RequestSpecHash: request.SpecHash, ResultHash: "validation-hash", GroupID: "payments", GroupName: "Payments", Status: ValidationCompatible, Targets: []TargetSnapshot{{InstanceUID: "collector-a"}}}
	approval, err := NewApproval(request, validation, "operator", "admin", "review security update")
	if err != nil { t.Fatal(err) }
	if approval.Status != ApprovalPending || approval.RequestSpecHash != request.SpecHash || approval.ValidationHash != validation.ResultHash || approval.TargetCount != 1 { t.Fatalf("unexpected approval: %#v", approval) }
	if _, err := NewApproval(request, validation, "operator", "operator", "self review"); err == nil { t.Fatal("self-review accepted") }
	validation.Status = ValidationAttention
	if _, err := NewApproval(request, validation, "operator", "admin", "review"); err == nil { t.Fatal("non-compatible validation accepted") }
}
