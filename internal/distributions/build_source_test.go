package distributions

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildSourceResolutionAllowsOnlyOfficialReleaseTags(t *testing.T) {
	manifest := BuildManifest{SourceRepository: OfficialContribRepository, SourceRef: "v0.140.0"}
	manifestJSON, _ := json.Marshal(manifest)
	execution := &BuildExecution{ID: "execution-1", BuildPlanHash: "plan-hash", ManifestJSON: string(manifestJSON)}
	result := &BuildWorkerResult{ID: "result-1", ExecutionID: execution.ID, BuildPlanHash: execution.BuildPlanHash, Status: BuildWorkerResultComplete}
	resolved, err := NewBuildSourceResolution(execution, result, "resolver-1", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != BuildSourceResolved || !strings.Contains(resolved.ArchiveURL, "/v0.140.0") {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}
	manifest.SourceRef = "main"
	manifestJSON, _ = json.Marshal(manifest)
	execution.ManifestJSON = string(manifestJSON)
	if _, err := NewBuildSourceResolution(execution, result, "resolver-1", time.Now()); err == nil || !strings.Contains(err.Error(), "release tag") {
		t.Fatalf("expected mutable ref rejection, got %v", err)
	}
	manifest.SourceRef = "v0.140.0"
	manifest.SourceRepository = "https://example.invalid/repo"
	manifestJSON, _ = json.Marshal(manifest)
	execution.ManifestJSON = string(manifestJSON)
	if _, err := NewBuildSourceResolution(execution, result, "resolver-1", time.Now()); err == nil || !strings.Contains(err.Error(), "allow-list") {
		t.Fatalf("expected repository rejection, got %v", err)
	}
}
