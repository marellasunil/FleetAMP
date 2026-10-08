package gitops

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type Worker struct {
	ID         string
	Executions storage.GitOpsExecutionStore
	Previews   storage.ComponentGitOpsPreviewStore
	Adapters   *Registry
	LeaseTTL   time.Duration
}

func (w *Worker) ProcessOne(ctx context.Context) error {
	now := time.Now().UTC()
	request, err := w.Executions.Claim(ctx, w.ID, now, now.Add(w.LeaseTTL))
	if err != nil {
		return err
	}
	status, message, evidence := lifecycle.GitOpsExecutionSucceeded, "", map[string]string{}
	preview, err := w.Previews.Get(ctx, request.PreviewID)
	if err == nil {
		var adapter Adapter
		adapter, err = w.Adapters.Adapter(request.Provider)
		if err == nil {
			var result Result
			result, err = adapter.Execute(ctx, request, preview)
			message, evidence = result.Message, result.Evidence
		}
	}
	if err != nil {
		status, message, evidence = lifecycle.GitOpsExecutionFailed, err.Error(), map[string]string{"error": err.Error()}
	}
	event, eventErr := lifecycle.NewGitOpsExecutionEvent(request.ID, status, w.ID, message, evidence)
	if eventErr != nil {
		return eventErr
	}
	return w.Executions.Complete(ctx, w.ID, event)
}

func (w *Worker) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		err := w.ProcessOne(ctx)
		if err != nil && !errors.Is(err, storage.ErrGitOpsExecutionQueueEmpty) && ctx.Err() == nil {
			slog.Error("GitOps worker failed", "component", "gitops_worker", "worker_id", w.ID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
