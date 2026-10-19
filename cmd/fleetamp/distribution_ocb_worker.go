package main

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"log/slog"
	"time"
)

type distributionOCBWorker struct {
	ID           string
	Inspections  storage.DistributionBuildSourceInspectionStore
	Artifacts    storage.DistributionBuildArtifactStore
	Runner       distributions.OCBRunner
	BuildTimeout time.Duration
}

func (w *distributionOCBWorker) runOnce(ctx context.Context) error {
	inspections, err := w.Inspections.List(ctx, 1000)
	if err != nil {
		return err
	}
	artifacts, err := w.Artifacts.List(ctx, 1000)
	if err != nil {
		return err
	}
	done := map[string]bool{}
	for _, item := range artifacts {
		done[item.InspectionID] = true
	}
	for _, inspection := range inspections {
		if done[inspection.ID] {
			continue
		}
		timeout := w.BuildTimeout
		if timeout <= 0 {
			timeout = 15 * time.Minute
		}
		buildCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := w.Runner.Build(buildCtx, inspection)
		cancel()
		if err != nil {
			return err
		}
		artifact, err := distributions.NewBuildArtifact(inspection, result, w.ID, time.Now())
		if err != nil {
			return err
		}
		if err := w.Artifacts.Create(ctx, artifact); err != nil {
			return err
		}
		done[inspection.ID] = true
		slog.Info("sandboxed Collector artifact generated", "component", "distribution_ocb_worker", "event", "artifact_generated", "artifact_id", artifact.ID, "artifact_sha256", artifact.ArtifactSHA256, "builder_image", artifact.BuilderImage)
	}
	return nil
}
func (w *distributionOCBWorker) run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("sandboxed Collector build failed", "component", "distribution_ocb_worker", "event", "ocb_build_failed", "worker_id", w.ID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
