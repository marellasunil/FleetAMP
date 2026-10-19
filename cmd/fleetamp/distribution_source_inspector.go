package main

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"log/slog"
	"time"
)

type distributionSourceInspector struct {
	ID, CacheDir string
	Acquisitions storage.DistributionBuildSourceAcquisitionStore
	Inspections  storage.DistributionBuildSourceInspectionStore
	Executions   storage.DistributionBuildExecutionStore
	Limits       distributions.ArchiveInspectionLimits
}

func (w *distributionSourceInspector) runOnce(ctx context.Context) error {
	acquisitions, err := w.Acquisitions.List(ctx, 1000)
	if err != nil {
		return err
	}
	inspections, err := w.Inspections.List(ctx, 1000)
	if err != nil {
		return err
	}
	done := map[string]bool{}
	for _, item := range inspections {
		done[item.AcquisitionID] = true
	}
	for _, acquisition := range acquisitions {
		if done[acquisition.ID] {
			continue
		}
		execution, err := w.Executions.Get(ctx, acquisition.ExecutionID)
		if err != nil {
			return err
		}
		item, err := distributions.InspectBuildSource(acquisition, execution, w.CacheDir, w.ID, w.Limits, time.Now())
		if err != nil {
			return err
		}
		if err := w.Inspections.Create(ctx, item); err != nil {
			return err
		}
		done[acquisition.ID] = true
		slog.Info("Collector source archive inspected and OCB manifest generated", "component", "distribution_source_inspector", "event", "source_inspected", "inspection_id", item.ID, "manifest_sha256", item.ManifestSHA256)
	}
	return nil
}
func (w *distributionSourceInspector) run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("Collector source inspection failed", "component", "distribution_source_inspector", "event", "source_inspection_failed", "worker_id", w.ID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
