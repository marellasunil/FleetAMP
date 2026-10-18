package distributions

import (
	"strings"
	"testing"
	"time"
)

func TestNewBuildExecutionRequiresExactApproval(t *testing.T) {
	plan := &BuildPlan{ID: "build-1", PlanHash: "hash-1", ManifestJSON: `{"distribution_id":"dist-1"}`}
	approved := &BuildApproval{ID: "approval-1", BuildPlanID: plan.ID, BuildPlanHash: plan.PlanHash, Decision: BuildDecisionApproved, ReviewedBy: "reviewer", ReviewedAt: time.Unix(2, 0)}
	execution, err := NewBuildExecution(plan, approved, "operator", time.Unix(3, 0))
	if err != nil {
		t.Fatal(err)
	}
	if execution.Status != BuildExecutionStatusQueued || execution.Executor != BuildExecutorDisabled {
		t.Fatalf("unexpected execution: %#v", execution)
	}

	rejected := *approved
	rejected.Decision = BuildDecisionRejected
	if _, err := NewBuildExecution(plan, &rejected, "operator", time.Now()); err == nil || !strings.Contains(err.Error(), "approved") {
		t.Fatalf("expected rejected decision error, got %v", err)
	}
	mismatch := *approved
	mismatch.BuildPlanHash = "different"
	if _, err := NewBuildExecution(plan, &mismatch, "operator", time.Now()); err == nil || !strings.Contains(err.Error(), "exact") {
		t.Fatalf("expected evidence mismatch, got %v", err)
	}
}
