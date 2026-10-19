package main

import (
	"context"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"strings"
	"testing"
)

type fakeOCBRunner struct{ calls int }

func (f *fakeOCBRunner) Build(_ context.Context, _ *distributions.BuildSourceInspection) (*distributions.OCBRunResult, error) {
	f.calls++
	return &distributions.OCBRunResult{Runtime: "podman", BuilderImage: "example.invalid/builder@sha256:" + strings.Repeat("a", 64), ArtifactReference: "artifact-cache://" + strings.Repeat("d", 64) + "/fleetamp-collector", ArtifactSHA256: strings.Repeat("d", 64), ArtifactSize: 10}, nil
}
func TestDistributionOCBWorkerBuildsEachInspectionOnce(t *testing.T) {
	ctx := context.Background()
	inspections := memory.NewDistributionBuildSourceInspectionStore()
	artifacts := memory.NewDistributionBuildArtifactStore()
	inspection := &distributions.BuildSourceInspection{ID: "inspection-1", ExecutionID: "execution-1", BuildPlanHash: strings.Repeat("b", 64), ManifestSHA256: strings.Repeat("c", 64), Status: distributions.BuildSourceInspected}
	if err := inspections.Create(ctx, inspection); err != nil {
		t.Fatal(err)
	}
	runner := &fakeOCBRunner{}
	worker := &distributionOCBWorker{ID: "worker", Inspections: inspections, Artifacts: artifacts, Runner: runner}
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
}
