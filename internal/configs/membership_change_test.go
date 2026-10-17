package configs

import "testing"

func TestNewMembershipChangeValidatesGovernedOperations(t *testing.T) {
	base := MembershipChange{Targets: []MembershipTarget{{AgentInstanceUID: "agent-1"}}, RequestedBy: "operator", AssignedReviewer: "reviewer", Reason: "move ownership"}
	assign := base
	assign.Operation = MembershipAssign
	assign.TargetGroupID = "payments"
	value, err := NewMembershipChange(assign)
	if err != nil {
		t.Fatal(err)
	}
	if value.ID == "" || value.Status != MembershipPending {
		t.Fatalf("unexpected request: %#v", value)
	}
	move := base
	move.Operation = MembershipMove
	move.SourceGroupID = "payments"
	move.TargetGroupID = "orders"
	if _, err := NewMembershipChange(move); err != nil {
		t.Fatal(err)
	}
	invalid := base
	invalid.Operation = MembershipMove
	invalid.SourceGroupID = "payments"
	invalid.TargetGroupID = "payments"
	if _, err := NewMembershipChange(invalid); err == nil {
		t.Fatal("expected same-group move rejection")
	}
}

func TestNewMembershipChangeRequiresReviewerAndReason(t *testing.T) {
	_, err := NewMembershipChange(MembershipChange{Operation: MembershipAssign, TargetGroupID: "payments", Targets: []MembershipTarget{{AgentInstanceUID: "agent-1"}}, RequestedBy: "operator"})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
