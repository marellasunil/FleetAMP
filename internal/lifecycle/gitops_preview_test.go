package lifecycle

import (
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"testing"
)

func TestPrepareGitOpsPreviewIsImmutableAndDeterministic(t *testing.T) {
	r, _ := NewRequest(Spec{Operation: Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security"}, "operator")
	p := &ExecutionPlan{ID: "plan", RequestID: r.ID, RequestSpecHash: r.SpecHash, ExecutorKind: ExecutorGitOps, PlanHash: "plan-hash", Targets: []TargetSnapshot{{InstanceUID: "a"}}}
	a, err := PrepareGitOpsPreview(p, r, "admin")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := PrepareGitOpsPreview(p, r, "admin")
	if a.PreviewHash != b.PreviewHash || a.RepositoryPath == "" || a.Diff == "" {
		t.Fatalf("unexpected previews: %#v %#v", a, b)
	}
}
