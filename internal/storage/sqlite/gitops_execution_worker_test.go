package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
)

func TestGitOpsExecutionClaimLeaseAndComplete(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "gitops-worker.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.db.SetMaxOpenConns(1)
	if _, err = db.db.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	store := db.GitOpsExecutions()
	now := time.Now().UTC()
	request := &lifecycle.GitOpsExecutionRequest{ID: "execution", ApprovalID: "approval", PreviewID: "preview", PreviewHash: "preview-hash", PlanHash: "plan-hash", ConnectionID: "connection", Provider: "github", Mode: "fleetamp-pull-request", RepositoryPath: "fleetamp/groups/payments/change.json", Branch: "main", RequestedBy: "admin", CreatedAt: now}
	queued := &lifecycle.GitOpsExecutionEvent{ID: "queued", ExecutionID: request.ID, Status: lifecycle.GitOpsExecutionQueued, Actor: "admin", Message: "queued", CreatedAt: now}
	if err = store.Create(ctx, request, queued); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(ctx, "worker-1", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != request.ID {
		t.Fatalf("claimed=%#v", claimed)
	}
	done, err := lifecycle.NewGitOpsExecutionEvent(request.ID, lifecycle.GitOpsExecutionSucceeded, "worker-1", "dry run complete", map[string]string{"dry_run": "true"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, "worker-1", done); err != nil {
		t.Fatal(err)
	}
	events, err := store.ListEvents(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 || events[1].Status != lifecycle.GitOpsExecutionClaimed || events[2].Status != lifecycle.GitOpsExecutionSucceeded {
		t.Fatalf("events=%#v", events)
	}
}
