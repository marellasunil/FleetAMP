package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

func TestBuildPreflightValidatesExactQueuedManifest(t *testing.T) {
	manifest := `{"distribution_id":"dist-1","distribution_spec_hash":"spec-1","collector_version":"0.140.0","source_repository":"https://github.com/open-telemetry/opentelemetry-collector-contrib","source_ref":"v0.140.0","components":{},"baseline_hash":"base-1","outputs":["oci-image"],"destination_ref":"oci://registry.example/collector:0.140.0","credential_ref":"secret://registry/platform"}`
	sum := sha256.Sum256([]byte(manifest))
	execution := &BuildExecution{ID: "execution-1", BuildPlanID: "plan-1", BuildPlanHash: hex.EncodeToString(sum[:]), ApprovalID: "approval-1", ApprovalReviewedBy: "reviewer", ManifestJSON: manifest, Status: BuildExecutionStatusQueued}
	receipt, err := NewBuildPreflight(execution, "operator", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != BuildPreflightReady || receipt.ManifestHash != execution.BuildPlanHash {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}

	tampered := *execution
	tampered.ManifestJSON = strings.Replace(manifest, "0.140.0", "0.141.0", 1)
	if _, err := NewBuildPreflight(&tampered, "operator", time.Now()); err == nil || !strings.Contains(err.Error(), "hash") {
		t.Fatalf("expected tamper rejection, got %v", err)
	}
}
