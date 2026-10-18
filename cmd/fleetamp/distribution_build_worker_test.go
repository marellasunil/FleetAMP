package main

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestDistributionBuildWorkerDryRunWritesOneImmutableResult(t *testing.T) {
	manifest := `{"distribution_id":"dist-1","distribution_spec_hash":"spec","collector_version":"0.140.0","source_repository":"https://github.com/open-telemetry/opentelemetry-collector-releases","source_ref":"v0.140.0","components":{},"baseline_hash":"base","outputs":["oci-image"],"destination_ref":"oci://registry/collector:0.140.0","credential_ref":"secret://registry/key"}`
	sum := sha256.Sum256([]byte(manifest))
	hash := hex.EncodeToString(sum[:])
	execution := &distributions.BuildExecution{ID: "execution-1", BuildPlanID: "plan-1", BuildPlanHash: hash, ApprovalID: "approval-1", ApprovalReviewedBy: "reviewer", ManifestJSON: manifest, Status: distributions.BuildExecutionStatusQueued}
	preflight := &distributions.BuildPreflight{ID: "preflight-1", ExecutionID: execution.ID, BuildPlanID: execution.BuildPlanID, BuildPlanHash: hash, ManifestHash: hash, Status: distributions.BuildPreflightReady, CheckedAt: time.Now()}
	executions := memory.NewDistributionBuildExecutionStore()
	preflights := memory.NewDistributionBuildPreflightStore()
	results := memory.NewDistributionBuildWorkerResultStore()
	sources := memory.NewDistributionBuildSourceStore()
	if err := executions.Create(t.Context(), execution); err != nil {
		t.Fatal(err)
	}
	if err := preflights.Create(t.Context(), preflight); err != nil {
		t.Fatal(err)
	}
	worker := &distributionBuildWorker{ID: "worker-1", Executions: executions, Preflights: preflights, Results: results, Sources: sources}
	if err := worker.runOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := worker.runOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	items, err := results.List(t.Context(), 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("results=%d err=%v", len(items), err)
	}
	if items[0].Adapter != distributions.BuildWorkerDryRun || items[0].Status != distributions.BuildWorkerResultComplete {
		t.Fatalf("unexpected result: %#v", items[0])
	}
	resolved, err := sources.List(t.Context(), 10)
	if err != nil || len(resolved) != 1 {
		t.Fatalf("sources=%d err=%v", len(resolved), err)
	}
	if resolved[0].Status != distributions.BuildSourceResolved {
		t.Fatalf("unexpected source: %#v", resolved[0])
	}
}
