package main

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

func int64EnvOrDefault(key string, fallback int64) int64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func boundedIntEnvOrDefault(key string, fallback, minimum, maximum int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < minimum || value > maximum {
		return fallback
	}
	return value
}

type distributionSourceDownloader struct {
	ID           string
	Sources      storage.DistributionBuildSourceStore
	Acquisitions storage.DistributionBuildSourceAcquisitionStore
	Fetcher      distributions.SourceFetcher
}

func (w *distributionSourceDownloader) runOnce(ctx context.Context) error {
	sources, err := w.Sources.List(ctx, 1000)
	if err != nil {
		return err
	}
	items, err := w.Acquisitions.List(ctx, 1000)
	if err != nil {
		return err
	}
	done := map[string]bool{}
	for _, item := range items {
		done[item.SourceResolutionID] = true
	}
	for _, source := range sources {
		if done[source.ID] {
			continue
		}
		sha, size, ref, err := w.Fetcher.Fetch(ctx, source)
		if err != nil {
			return err
		}
		item, err := distributions.NewBuildSourceAcquisition(source, sha, size, ref, w.ID, time.Now())
		if err != nil {
			return err
		}
		if err := w.Acquisitions.Create(ctx, item); err != nil {
			return err
		}
		done[source.ID] = true
		slog.Info("official Collector source acquired", "component", "distribution_source_downloader", "event", "source_acquired", "source_ref", source.SourceRef, "archive_sha256", sha, "archive_size", size)
	}
	return nil
}
func (w *distributionSourceDownloader) run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("official Collector source acquisition failed", "component", "distribution_source_downloader", "event", "source_acquisition_failed", "worker_id", w.ID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
