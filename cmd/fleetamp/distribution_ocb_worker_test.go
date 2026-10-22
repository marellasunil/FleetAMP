package main

import (
	"context"
	"errors"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"strings"
	"testing"
	"time"
)

type fakeOCBRunner struct {
	calls  int
	err    error
	result *distributions.OCBRunResult
}

func (f *fakeOCBRunner) Build(_ context.Context, _ *distributions.BuildSourceInspection) (*distributions.OCBRunResult, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &distributions.OCBRunResult{Runtime: "podman", BuilderImage: "example.invalid/builder@sha256:" + strings.Repeat("a", 64), ArtifactReference: "artifact-cache://" + strings.Repeat("d", 64) + "/fleetamp-collector", ArtifactSHA256: strings.Repeat("d", 64), ArtifactSize: 10}, nil
}

func TestDistributionOCBWorkerRecordsInvalidArtifactEvidenceAsFailure(t *testing.T) {
	ctx := context.Background()
	inspections := memory.NewDistributionBuildSourceInspectionStore()
	attempts := memory.NewDistributionBuildAttemptStore()
	inspection := &distributions.BuildSourceInspection{ID: "inspection-1", ExecutionID: "execution-1", BuildPlanHash: strings.Repeat("b", 64), ManifestSHA256: strings.Repeat("c", 64), Status: distributions.BuildSourceInspected}
	if err := inspections.Create(ctx, inspection); err != nil {
		t.Fatal(err)
	}
	runner := &fakeOCBRunner{result: &distributions.OCBRunResult{Runtime: "podman", BuilderImage: "unpinned:latest", ArtifactReference: "bad", ArtifactSHA256: "bad", ArtifactSize: 1}}
	worker := &distributionOCBWorker{ID: "worker", Inspections: inspections, Artifacts: memory.NewDistributionBuildArtifactStore(), Attempts: attempts, Retries: memory.NewDistributionBuildRetryRequestStore(), Runner: runner, MaxAutomaticAttempts: 1}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	receipts, _ := attempts.List(ctx, 10)
	if len(receipts) != 1 || receipts[0].FailureCode != distributions.BuildFailureArtifact {
		t.Fatalf("receipts=%#v", receipts)
	}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatal("invalid artifact evidence retried beyond bound")
	}
}
func TestDistributionOCBWorkerBuildsEachInspectionOnce(t *testing.T) {
	ctx := context.Background()
	inspections := memory.NewDistributionBuildSourceInspectionStore()
	artifacts := memory.NewDistributionBuildArtifactStore()
	attempts := memory.NewDistributionBuildAttemptStore()
	retries := memory.NewDistributionBuildRetryRequestStore()
	inspection := &distributions.BuildSourceInspection{ID: "inspection-1", ExecutionID: "execution-1", BuildPlanHash: strings.Repeat("b", 64), ManifestSHA256: strings.Repeat("c", 64), Status: distributions.BuildSourceInspected}
	if err := inspections.Create(ctx, inspection); err != nil {
		t.Fatal(err)
	}
	runner := &fakeOCBRunner{}
	worker := &distributionOCBWorker{ID: "worker", Inspections: inspections, Artifacts: artifacts, Attempts: attempts, Retries: retries, Runner: runner}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("runner calls=%d", runner.calls)
	}
	items, err := artifacts.List(ctx, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("artifacts=%d err=%v", len(items), err)
	}
	receipts, _ := attempts.List(ctx, 10)
	if len(receipts) != 1 || receipts[0].Status != distributions.BuildAttemptSucceeded {
		t.Fatalf("attempts=%#v", receipts)
	}
}

func TestDistributionOCBWorkerBoundsFailuresAndAllowsManualRetry(t *testing.T) {
	ctx := context.Background()
	inspections := memory.NewDistributionBuildSourceInspectionStore()
	artifacts := memory.NewDistributionBuildArtifactStore()
	attempts := memory.NewDistributionBuildAttemptStore()
	retries := memory.NewDistributionBuildRetryRequestStore()
	inspection := &distributions.BuildSourceInspection{ID: "inspection-1", ExecutionID: "execution-1", BuildPlanHash: strings.Repeat("b", 64), ManifestSHA256: strings.Repeat("c", 64), Status: distributions.BuildSourceInspected}
	if err := inspections.Create(ctx, inspection); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 19, 20, 0, 0, 0, time.UTC)
	runner := &fakeOCBRunner{err: errors.New("unsafe\x00 runner failure")}
	worker := &distributionOCBWorker{ID: "worker", Inspections: inspections, Artifacts: artifacts, Attempts: attempts, Retries: retries, Runner: runner, MaxAutomaticAttempts: 2, RetryBackoff: time.Minute, Now: func() time.Time { return now }}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatalf("calls=%d", runner.calls)
	}
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 {
		t.Fatal("worker ignored retry backoff")
	}
	now = now.Add(time.Minute)
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 2 {
		t.Fatalf("calls=%d", runner.calls)
	}
	now = now.Add(time.Hour)
	if err := worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 2 {
		t.Fatal("automatic retry limit was not enforced")
	}
	receipts, _ := attempts.List(ctx, 10)
	latest := receipts[0]
	if latest.AttemptNumber != 2 || latest.NextRetryAt.IsZero() == false || strings.Contains(latest.FailureMessage, "\x00") {
		t.Fatalf("latest=%#v", latest)
	}
	request, err := distributions.NewBuildRetryRequest(inspection, latest, "administrator reviewed failure", "admin", now)
	if err != nil {
		t.Fatal(err)
	}
	if err = retries.Create(ctx, request); err != nil {
		t.Fatal(err)
	}
	runner.err = nil
	if err = worker.runOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 3 {
		t.Fatalf("manual retry calls=%d", runner.calls)
	}
	receipts, _ = attempts.List(ctx, 10)
	if receipts[0].Trigger != distributions.BuildTriggerManual || receipts[0].RetryRequestID != request.ID || receipts[0].Status != distributions.BuildAttemptSucceeded {
		t.Fatalf("manual receipt=%#v", receipts[0])
	}
}
