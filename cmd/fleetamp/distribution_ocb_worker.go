package main

import (
	"context"
	"errors"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"log/slog"
	"sort"
	"time"
)

type distributionOCBWorker struct {
	ID                   string
	Inspections          storage.DistributionBuildSourceInspectionStore
	Artifacts            storage.DistributionBuildArtifactStore
	Attempts             storage.DistributionBuildAttemptStore
	Retries              storage.DistributionBuildRetryRequestStore
	Runner               distributions.OCBRunner
	BuildTimeout         time.Duration
	RetryBackoff         time.Duration
	MaxAutomaticAttempts int
	Now                  func() time.Time
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
	attempts, err := w.Attempts.List(ctx, 10000)
	if err != nil {
		return err
	}
	retries, err := w.Retries.List(ctx, 10000)
	if err != nil {
		return err
	}
	byInspection := map[string][]*distributions.BuildAttempt{}
	consumedRetries := map[string]bool{}
	for _, attempt := range attempts {
		byInspection[attempt.InspectionID] = append(byInspection[attempt.InspectionID], attempt)
		if attempt.RetryRequestID != "" {
			consumedRetries[attempt.RetryRequestID] = true
		}
	}
	for _, list := range byInspection {
		sort.Slice(list, func(i, j int) bool { return list[i].AttemptNumber < list[j].AttemptNumber })
	}
	retryByInspection := map[string]*distributions.BuildRetryRequest{}
	for _, request := range retries {
		if consumedRetries[request.ID] {
			continue
		}
		if current := retryByInspection[request.InspectionID]; current == nil || request.RequestedAt.After(current.RequestedAt) {
			retryByInspection[request.InspectionID] = request
		}
	}
	for _, inspection := range inspections {
		if done[inspection.ID] {
			continue
		}
		now := time.Now().UTC()
		if w.Now != nil {
			now = w.Now().UTC()
		}
		prior := byInspection[inspection.ID]
		trigger, retryRequestID := distributions.BuildTriggerAutomatic, ""
		maxAutomatic := w.MaxAutomaticAttempts
		if maxAutomatic <= 0 {
			maxAutomatic = 3
		}
		if maxAutomatic > 10 {
			maxAutomatic = 10
		}
		automaticCount := 0
		for _, item := range prior {
			if item.Trigger == distributions.BuildTriggerAutomatic {
				automaticCount++
			}
		}
		if len(prior) > 0 {
			latest := prior[len(prior)-1]
			request := retryByInspection[inspection.ID]
			if request != nil && request.PreviousAttemptID == latest.ID {
				trigger = distributions.BuildTriggerManual
				retryRequestID = request.ID
			} else if latest.Trigger == distributions.BuildTriggerManual || automaticCount >= maxAutomatic || (!latest.NextRetryAt.IsZero() && now.Before(latest.NextRetryAt)) {
				continue
			}
		}
		timeout := w.BuildTimeout
		if timeout <= 0 {
			timeout = 15 * time.Minute
		}
		startedAt := now
		buildCtx, cancel := context.WithTimeout(ctx, timeout)
		result, err := w.Runner.Build(buildCtx, inspection)
		cancel()
		if err != nil {
			completedAt := time.Now().UTC()
			if w.Now != nil {
				completedAt = w.Now().UTC()
			}
			failureCode := distributions.BuildFailureRunner
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(buildCtx.Err(), context.DeadlineExceeded) {
				failureCode = distributions.BuildFailureTimeout
			}
			nextRetryAt := automaticRetryAt(trigger, automaticCount, maxAutomatic, completedAt, w.RetryBackoff)
			attempt, receiptErr := distributions.NewFailedBuildAttempt(inspection, len(prior)+1, trigger, retryRequestID, failureCode, err.Error(), w.ID, startedAt, completedAt, nextRetryAt)
			if receiptErr != nil {
				return receiptErr
			}
			if receiptErr = w.Attempts.Create(ctx, attempt); receiptErr != nil {
				return receiptErr
			}
			byInspection[inspection.ID] = append(prior, attempt)
			slog.Warn("sandboxed Collector build attempt failed", "component", "distribution_ocb_worker", "event", "ocb_build_attempt_failed", "attempt_id", attempt.ID, "attempt_number", attempt.AttemptNumber, "failure_code", attempt.FailureCode, "next_retry_at", attempt.NextRetryAt)
			continue
		}
		completedAt := time.Now().UTC()
		if w.Now != nil {
			completedAt = w.Now().UTC()
		}
		artifact, err := distributions.NewBuildArtifact(inspection, result, w.ID, completedAt)
		if err != nil {
			nextRetryAt := automaticRetryAt(trigger, automaticCount, maxAutomatic, completedAt, w.RetryBackoff)
			attempt, receiptErr := distributions.NewFailedBuildAttempt(inspection, len(prior)+1, trigger, retryRequestID, distributions.BuildFailureArtifact, err.Error(), w.ID, startedAt, completedAt, nextRetryAt)
			if receiptErr != nil {
				return receiptErr
			}
			if receiptErr = w.Attempts.Create(ctx, attempt); receiptErr != nil {
				return receiptErr
			}
			byInspection[inspection.ID] = append(prior, attempt)
			slog.Warn("sandboxed Collector artifact validation failed", "component", "distribution_ocb_worker", "event", "ocb_artifact_validation_failed", "attempt_id", attempt.ID, "attempt_number", attempt.AttemptNumber, "next_retry_at", attempt.NextRetryAt)
			continue
		}
		if err := w.Artifacts.Create(ctx, artifact); err != nil {
			return err
		}
		attempt, err := distributions.NewSuccessfulBuildAttempt(inspection, artifact, len(prior)+1, trigger, retryRequestID, w.ID, startedAt, completedAt)
		if err != nil {
			return err
		}
		if err = w.Attempts.Create(ctx, attempt); err != nil {
			return err
		}
		done[inspection.ID] = true
		slog.Info("sandboxed Collector artifact generated", "component", "distribution_ocb_worker", "event", "artifact_generated", "attempt_id", attempt.ID, "artifact_id", artifact.ID, "artifact_sha256", artifact.ArtifactSHA256, "builder_image", artifact.BuilderImage)
	}
	return nil
}

func automaticRetryAt(trigger string, automaticCount, maxAutomatic int, completedAt time.Time, configured time.Duration) time.Time {
	if trigger != distributions.BuildTriggerAutomatic || automaticCount+1 >= maxAutomatic {
		return time.Time{}
	}
	backoff := configured
	if backoff <= 0 {
		backoff = time.Minute
	}
	for i := 1; i < automaticCount+1; i++ {
		if backoff >= 30*time.Minute {
			return completedAt.Add(30 * time.Minute)
		}
		backoff *= 2
	}
	if backoff > 30*time.Minute {
		backoff = 30 * time.Minute
	}
	return completedAt.Add(backoff)
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
