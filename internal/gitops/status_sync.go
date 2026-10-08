package gitops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type StatusSynchronizer struct {
	ID         string
	Executions storage.GitOpsExecutionStore
	Previews   storage.ComponentGitOpsPreviewStore
	Adapters   *Registry
}

func (s *StatusSynchronizer) ProcessAll(ctx context.Context) (int, error) {
	if s == nil || strings.TrimSpace(s.ID) == "" || s.Executions == nil || s.Previews == nil || s.Adapters == nil {
		return 0, errors.New("GitOps status synchronizer is not configured")
	}
	requests, err := s.Executions.List(ctx, 500)
	if err != nil {
		return 0, err
	}
	updated := 0
	errorsSeen := []error{}
	for _, request := range requests {
		events, listErr := s.Executions.ListEvents(ctx, request.ID)
		if listErr != nil {
			errorsSeen = append(errorsSeen, listErr)
			continue
		}
		if len(events) == 0 {
			continue
		}
		latest := events[len(events)-1]
		if latest.Status != lifecycle.GitOpsExecutionSucceeded && latest.Status != lifecycle.GitOpsChangeOpen {
			continue
		}
		evidence := providerChangeEvidence(events)
		if evidence["pull_request_number"] == "" {
			continue
		}
		preview, getErr := s.Previews.Get(ctx, request.PreviewID)
		if getErr != nil {
			errorsSeen = append(errorsSeen, getErr)
			continue
		}
		adapter, adapterErr := s.Adapters.StatusAdapter(request.Provider)
		if adapterErr != nil {
			continue
		}
		result, statusErr := adapter.Status(ctx, request, preview, evidence)
		if statusErr != nil {
			errorsSeen = append(errorsSeen, fmt.Errorf("execution %s: %w", request.ID, statusErr))
			continue
		}
		if result.Status == latest.Status {
			continue
		}
		event, eventErr := lifecycle.NewGitOpsExecutionEvent(request.ID, result.Status, s.ID, result.Message, result.Evidence)
		if eventErr != nil {
			errorsSeen = append(errorsSeen, eventErr)
			continue
		}
		if appendErr := s.Executions.AppendEvent(ctx, event); appendErr != nil {
			errorsSeen = append(errorsSeen, appendErr)
			continue
		}
		updated++
	}
	return updated, errors.Join(errorsSeen...)
}

func providerChangeEvidence(events []*lifecycle.GitOpsExecutionEvent) map[string]string {
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Evidence["pull_request_number"] != "" {
			result := make(map[string]string, len(events[index].Evidence))
			for key, value := range events[index].Evidence {
				result[key] = value
			}
			return result
		}
	}
	return map[string]string{}
}

func (s *StatusSynchronizer) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if updated, err := s.ProcessAll(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("GitOps provider status synchronization completed with errors", "component", "gitops_status_sync", "synchronizer_id", s.ID, "updated", updated, "error", err)
		} else if updated > 0 {
			slog.Info("GitOps provider statuses synchronized", "component", "gitops_status_sync", "synchronizer_id", s.ID, "updated", updated)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
