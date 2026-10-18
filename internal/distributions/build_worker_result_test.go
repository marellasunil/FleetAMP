package distributions

import (
	"strings"
	"testing"
	"time"
)

func TestDryRunWorkerRequiresMatchingPreflightEvidence(t *testing.T) {
	execution := &BuildExecution{ID: "execution-1", BuildPlanID: "plan-1", BuildPlanHash: "hash", ManifestJSON: `{}`}
	preflight := &BuildPreflight{ID: "preflight-1", ExecutionID: execution.ID, BuildPlanID: execution.BuildPlanID, BuildPlanHash: execution.BuildPlanHash, ManifestHash: execution.BuildPlanHash, Status: BuildPreflightReady}
	if _, err := NewDryRunBuildWorkerResult(execution, preflight, "worker-1", time.Now()); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("expected hash rejection, got %v", err)
	}
}
