package gitops

import (
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestWorkerProcessesApprovedExecutionWithNoopAdapter(t *testing.T) {
	executions := memory.NewGitOpsExecutionStore()
	previews := memory.NewComponentGitOpsPreviewStore()
	preview := &lifecycle.GitOpsPreview{ID: "preview", PlanID: "plan", PlanHash: "plan-hash", PreviewHash: "preview-hash", RepositoryPath: "fleetamp/groups/payments/change.json", Connection: lifecycle.GitConnectionSnapshot{ID: "connection", Provider: "github", Mode: "fleetamp-pull-request", Branch: "main"}, Files: []lifecycle.GitOpsFile{{Path: "change.json", Content: "{}"}}, CreatedAt: time.Now().UTC()}
	if err := previews.Create(t.Context(), preview); err != nil {
		t.Fatal(err)
	}
	request := &lifecycle.GitOpsExecutionRequest{ID: "execution", ApprovalID: "approval", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, PlanHash: preview.PlanHash, ConnectionID: preview.Connection.ID, Provider: "github", Mode: "fleetamp-pull-request", RepositoryPath: preview.RepositoryPath, Branch: "main", RequestedBy: "admin", CreatedAt: time.Now().UTC()}
	queued := &lifecycle.GitOpsExecutionEvent{ID: "queued", ExecutionID: request.ID, Status: lifecycle.GitOpsExecutionQueued, Actor: "admin", Message: "queued", CreatedAt: time.Now().UTC()}
	if err := executions.Create(t.Context(), request, queued); err != nil {
		t.Fatal(err)
	}
	worker := &Worker{ID: "worker-1", Executions: executions, Previews: previews, Adapters: NewDryRunRegistry(), LeaseTTL: time.Minute}
	if err := worker.ProcessOne(t.Context()); err != nil {
		t.Fatal(err)
	}
	events, err := executions.ListEvents(t.Context(), request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Status != lifecycle.GitOpsExecutionClaimed || events[2].Status != lifecycle.GitOpsExecutionSucceeded || events[2].Evidence["dry_run"] != "true" {
		t.Fatalf("unexpected events: %#v", events)
	}
}
