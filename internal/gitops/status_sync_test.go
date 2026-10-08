package gitops

import (
	"context"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

type fixedStatusAdapter struct {
	result StatusResult
	calls  int
}

func (a *fixedStatusAdapter) Execute(context.Context, *lifecycle.GitOpsExecutionRequest, *lifecycle.GitOpsPreview) (Result, error) {
	return Result{}, nil
}

func (a *fixedStatusAdapter) Status(_ context.Context, _ *lifecycle.GitOpsExecutionRequest, _ *lifecycle.GitOpsPreview, evidence map[string]string) (StatusResult, error) {
	a.calls++
	if evidence["pull_request_number"] != "42" {
		return StatusResult{}, context.Canceled
	}
	return a.result, nil
}

func TestStatusSynchronizerAppendsTransitionOnlyOnce(t *testing.T) {
	executions := memory.NewGitOpsExecutionStore()
	previews := memory.NewComponentGitOpsPreviewStore()
	preview := &lifecycle.GitOpsPreview{ID: "preview", PreviewHash: "preview-hash", Connection: lifecycle.GitConnectionSnapshot{ID: "connection", Provider: "github"}, CreatedAt: time.Now().UTC()}
	if err := previews.Create(t.Context(), preview); err != nil {
		t.Fatal(err)
	}
	request := &lifecycle.GitOpsExecutionRequest{ID: "execution", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, ConnectionID: preview.Connection.ID, Provider: "github", CreatedAt: time.Now().UTC()}
	queued := &lifecycle.GitOpsExecutionEvent{ID: "queued", ExecutionID: request.ID, Status: lifecycle.GitOpsExecutionQueued, Actor: "admin", Message: "queued", CreatedAt: time.Now().UTC()}
	if err := executions.Create(t.Context(), request, queued); err != nil {
		t.Fatal(err)
	}
	succeeded, _ := lifecycle.NewGitOpsExecutionEvent(request.ID, lifecycle.GitOpsExecutionSucceeded, "worker", "pull request created", map[string]string{"pull_request_number": "42", "pull_request_url": "https://github.example/pull/42"})
	if err := executions.AppendEvent(t.Context(), succeeded); err != nil {
		t.Fatal(err)
	}
	adapter := &fixedStatusAdapter{result: StatusResult{Status: lifecycle.GitOpsChangeMerged, Message: "GitHub pull request was merged", Evidence: map[string]string{"pull_request_number": "42", "merge_commit_sha": "merged-sha"}}}
	registry := &Registry{adapters: map[string]Adapter{"github": adapter}}
	synchronizer := &StatusSynchronizer{ID: "status-1", Executions: executions, Previews: previews, Adapters: registry}
	updated, err := synchronizer.ProcessAll(t.Context())
	if err != nil || updated != 1 {
		t.Fatalf("updated=%d err=%v", updated, err)
	}
	updated, err = synchronizer.ProcessAll(t.Context())
	if err != nil || updated != 0 {
		t.Fatalf("second updated=%d err=%v", updated, err)
	}
	events, _ := executions.ListEvents(t.Context(), request.ID)
	latest := events[len(events)-1]
	if len(events) != 3 || latest.Status != lifecycle.GitOpsChangeMerged || latest.Evidence["merge_commit_sha"] != "merged-sha" || adapter.calls != 1 {
		t.Fatalf("events=%#v calls=%d", events, adapter.calls)
	}
}
