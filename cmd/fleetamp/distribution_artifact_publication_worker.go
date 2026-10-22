package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type distributionArtifactPublicationWorker struct {
	ID         string
	Executions storage.DistributionArtifactPublicationExecutionStore
	Publisher  distributions.ArtifactRegistryPublisher
	LeaseTTL   time.Duration
	Timeout    time.Duration
	Now        func() time.Time
}

func (w *distributionArtifactPublicationWorker) runOnce(ctx context.Context) error {
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now().UTC()
	}
	timeout := w.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	leaseTTL := w.LeaseTTL
	if leaseTTL <= 0 {
		leaseTTL = 20 * time.Minute
	}
	if leaseTTL <= timeout {
		leaseTTL = timeout + 5*time.Minute
	}
	work, err := w.Executions.Claim(ctx, w.ID, now, now.Add(leaseTTL))
	if errors.Is(err, storage.ErrArtifactPublicationQueueEmpty) {
		return nil
	}
	if err != nil {
		return err
	}
	publishCtx, cancel := context.WithTimeout(ctx, timeout)
	result, publishErr := w.Publisher.Publish(publishCtx, work.Preflight)
	cancel()
	completedAt := time.Now().UTC()
	if w.Now != nil {
		completedAt = w.Now().UTC()
	}
	var receipt *distributions.ArtifactPublicationExecution
	if publishErr == nil {
		receipt, err = distributions.NewSuccessfulArtifactPublicationExecution(work.Publication, work.Preflight, result, work.AttemptNumber, work.Trigger, work.RetryRequestID, w.ID, now, completedAt)
	} else {
		failureCode := distributions.ArtifactPublicationFailurePublisher
		if errors.Is(publishErr, context.DeadlineExceeded) || errors.Is(publishCtx.Err(), context.DeadlineExceeded) {
			failureCode = distributions.ArtifactPublicationFailureTimeout
		}
		receipt, err = distributions.NewFailedArtifactPublicationExecution(work.Publication, work.Preflight, failureCode, publishErr.Error(), work.AttemptNumber, work.Trigger, work.RetryRequestID, w.ID, now, completedAt)
	}
	if err != nil {
		return err
	}
	if err = w.Executions.Complete(ctx, w.ID, receipt); err != nil {
		return err
	}
	if receipt.Status == distributions.ArtifactPublicationSucceeded {
		slog.Info("Collector artifact published", "component", "distribution_artifact_publisher", "event", "artifact_published", "publication_id", receipt.PublicationID, "artifact_sha256", receipt.ArtifactSHA256, "destination", receipt.DestinationRef, "evidence_hash", receipt.EvidenceHash)
		return nil
	}
	slog.Error("Collector artifact publication failed", "component", "distribution_artifact_publisher", "event", "artifact_publication_failed", "publication_id", receipt.PublicationID, "failure_code", receipt.FailureCode, "failure_digest", receipt.FailureDigest)
	return nil
}

func (w *distributionArtifactPublicationWorker) run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := w.runOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Error("artifact publication worker failed", "component", "distribution_artifact_publisher", "worker_id", w.ID, "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
