package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type distributionBuildWorker struct {
	ID         string
	Executions storage.DistributionBuildExecutionStore
	Preflights storage.DistributionBuildPreflightStore
	Results    storage.DistributionBuildWorkerResultStore
}

func (w *distributionBuildWorker) runOnce(ctx context.Context) error {
	preflights, err := w.Preflights.List(ctx, 1000)
	if err != nil {
		return err
	}
	results, err := w.Results.List(ctx, 1000)
	if err != nil {
		return err
	}
	completed := map[string]bool{}
	for _, result := range results {
		completed[result.ExecutionID] = true
	}
	for _, preflight := range preflights {
		if completed[preflight.ExecutionID] {
			continue
		}
		execution, err := w.Executions.Get(ctx, preflight.ExecutionID)
		if err != nil {
			return err
		}
		result, err := distributions.NewDryRunBuildWorkerResult(execution, preflight, w.ID, time.Now())
		if err != nil {
			return err
		}
		if err := w.Results.Create(ctx, result); err != nil {
			return err
		}
		completed[preflight.ExecutionID] = true
		slog.Info("Collector build dry-run completed", "component", "distribution_build_worker", "event", "dry_run_completed", "execution_id", execution.ID, "worker_id", w.ID, "evidence_hash", result.EvidenceHash)
	}
	return nil
}

func (w *distributionBuildWorker) run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("Collector build dry-run failed", "component", "distribution_build_worker", "event", "dry_run_failed", "worker_id", w.ID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
