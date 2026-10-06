package lifecycle

import (
	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"testing"
)

func TestPrepareGitOpsPreviewIsImmutableAndDeterministic(t *testing.T) {
	r, _ := NewRequest(Spec{Operation: Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security"}, "operator")
	p := &ExecutionPlan{ID: "plan", RequestID: r.ID, RequestSpecHash: r.SpecHash, ExecutorKind: ExecutorGitOps, PlanHash: "plan-hash", Targets: []TargetSnapshot{{InstanceUID: "a"}}}
	c := &integrations.Connection{ID: "connection", Name: "production", Provider: integrations.GitHub, Organization: "acme", Repository: "telemetry", Branch: "main", AllowedRoot: "fleetamp/groups", Mode: integrations.ModePullRequest, GroupIDs: []string{"payments"}, Enabled: true}
	a, err := PrepareGitOpsPreview(p, r, c, "admin")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := PrepareGitOpsPreview(p, r, c, "admin")
	if a.PreviewHash != b.PreviewHash || a.RepositoryPath != "fleetamp/groups/payments/components/"+r.ID+".json" || a.Diff == "" || a.Connection.ID != c.ID {
		t.Fatalf("unexpected previews: %#v %#v", a, b)
	}
}

func TestPrepareGitOpsPreviewRejectsConnectionOutsideGroup(t *testing.T) {
	r, _ := NewRequest(Spec{Operation: Install, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", DesiredVersion: "0.149.0", Reason: "new gateway"}, "operator")
	p := &ExecutionPlan{ID: "plan", RequestID: r.ID, RequestSpecHash: r.SpecHash, ExecutorKind: ExecutorGitOps, PlanHash: "plan-hash"}
	c := &integrations.Connection{ID: "connection", AllowedRoot: "fleetamp/groups", GroupIDs: []string{"orders"}, Enabled: true}
	if _, err := PrepareGitOpsPreview(p, r, c, "admin"); err == nil {
		t.Fatal("expected group authorization failure")
	}
	c.GroupIDs = []string{"payments"}
	c.Enabled = false
	if _, err := PrepareGitOpsPreview(p, r, c, "admin"); err == nil {
		t.Fatal("expected disabled connection failure")
	}
}

func TestPrepareGitOpsPreviewRejectsPathTraversalInGroupID(t *testing.T) {
	r, _ := NewRequest(Spec{Operation: Install, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "../escape", DeploymentMethod: "gitops", DesiredVersion: "0.149.0", Reason: "new gateway"}, "operator")
	p := &ExecutionPlan{ID: "plan", RequestID: r.ID, RequestSpecHash: r.SpecHash, ExecutorKind: ExecutorGitOps, PlanHash: "plan-hash"}
	c := &integrations.Connection{ID: "connection", AllowedRoot: "fleetamp/groups", GroupIDs: []string{"../escape"}, Enabled: true}
	if _, err := PrepareGitOpsPreview(p, r, c, "admin"); err == nil {
		t.Fatal("expected repository path traversal failure")
	}
}
