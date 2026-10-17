package sqlite

import (
	"context"
	"errors"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"path/filepath"
	"testing"
)

func TestMembershipChangePersistenceAndStateGuard(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	value, err := configs.NewMembershipChange(configs.MembershipChange{Operation: configs.MembershipAssign, TargetGroupID: "payments", TargetGroupName: "Payments", Targets: []configs.MembershipTarget{{AgentInstanceUID: "agent-1", AgentName: "collector-1", PreviousConfigurationID: "base-1"}}, RequestedBy: "operator", AssignedReviewer: "admin", Reason: "onboard"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MembershipChanges().Create(ctx, value); err != nil {
		t.Fatal(err)
	}
	got, err := db.MembershipChanges().Get(ctx, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TargetGroupID != "payments" || len(got.Targets) != 1 || got.Targets[0].PreviousConfigurationID != "base-1" {
		t.Fatalf("unexpected persisted request: %#v", got)
	}
	if err := db.MembershipChanges().Update(ctx, value.ID, configs.MembershipPending, configs.MembershipExecuting, "admin", "approved", ""); err != nil {
		t.Fatal(err)
	}
	results := []configs.MembershipExecutionResult{{AgentInstanceUID: "agent-1", AgentName: "collector-1", Status: configs.MembershipTargetComplete, ConfigurationID: "config-1"}}
	if err := db.MembershipChanges().SaveExecutionResults(ctx, value.ID, results); err != nil {
		t.Fatal(err)
	}
	got, err = db.MembershipChanges().Get(ctx, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ExecutionResults) != 1 || got.ExecutionResults[0].Status != configs.MembershipTargetComplete || got.ExecutionResults[0].ConfigurationID != "config-1" {
		t.Fatalf("unexpected execution evidence: %#v", got.ExecutionResults)
	}
	if err := db.MembershipChanges().Update(ctx, value.ID, configs.MembershipPending, configs.MembershipRejected, "admin", "late", ""); !errors.Is(err, storage.ErrMembershipChangeConflict) {
		t.Fatalf("expected state conflict, got %v", err)
	}
}
