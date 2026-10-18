package distributions

import (
	"testing"
	"time"
)

func TestBuildApprovalBindsExactPlanAndEnforcesSeparationOfDuties(t *testing.T) {
	plan := &BuildPlan{ID: "build-1", PlanHash: "sha256-plan", RequestedBy: "requester"}
	approval, err := NewBuildApproval(plan, BuildDecisionApproved, "Reviewed immutable inputs", "reviewer", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if approval.BuildPlanHash != plan.PlanHash {
		t.Fatal("exact plan hash was not bound")
	}
	if _, err := NewBuildApproval(plan, BuildDecisionApproved, "self approval", "requester", time.Now()); err == nil {
		t.Fatal("expected self-approval rejection")
	}
}
func TestBuildApprovalRequiresReasonAndFinalDecision(t *testing.T) {
	plan := &BuildPlan{ID: "build-1", PlanHash: "hash", RequestedBy: "requester"}
	if _, err := NewBuildApproval(plan, "pending", "reason", "reviewer", time.Now()); err == nil {
		t.Fatal("expected invalid decision rejection")
	}
	if _, err := NewBuildApproval(plan, BuildDecisionRejected, "", "reviewer", time.Now()); err == nil {
		t.Fatal("expected missing reason rejection")
	}
}
