package distributions

import (
	"strings"
	"testing"
	"time"
)

func testInspectedSource() *BuildSourceInspection {
	return &BuildSourceInspection{ID: "inspection-1", ExecutionID: "execution-1", BuildPlanHash: strings.Repeat("a", 64), ManifestSHA256: strings.Repeat("b", 64), Status: BuildSourceInspected}
}

func TestFailedBuildAttemptSanitizesAndBindsEvidence(t *testing.T) {
	now := time.Date(2026, 10, 19, 20, 0, 0, 0, time.UTC)
	attempt, err := NewFailedBuildAttempt(testInspectedSource(), 1, BuildTriggerAutomatic, "", BuildFailureRunner, "failed\x00 token=top-secret Authorization=credential Bearer abc123\nnext", "worker", now, now.Add(time.Second), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(attempt.FailureMessage, "top-secret") || strings.Contains(attempt.FailureMessage, "credential") || strings.Contains(attempt.FailureMessage, "abc123") || strings.ContainsRune(attempt.FailureMessage, '\x00') {
		t.Fatalf("unsafe failure receipt: %q", attempt.FailureMessage)
	}
	if len(attempt.EvidenceHash) != 64 || attempt.ID == "" {
		t.Fatalf("invalid evidence: %#v", attempt)
	}
}

func TestManualAttemptRequiresRetryAuthorization(t *testing.T) {
	inspection := testInspectedSource()
	artifact := &BuildArtifact{InspectionID: inspection.ID, ID: "artifact-1", ArtifactSHA256: strings.Repeat("c", 64)}
	now := time.Now()
	if _, err := NewSuccessfulBuildAttempt(inspection, artifact, 2, BuildTriggerManual, "", "worker", now, now); err == nil {
		t.Fatal("manual attempt accepted without retry request")
	}
}

func TestRetryRequestRequiresFailedLatestAttempt(t *testing.T) {
	now := time.Now()
	inspection := testInspectedSource()
	artifact := &BuildArtifact{InspectionID: inspection.ID, ID: "artifact-1", ArtifactSHA256: strings.Repeat("c", 64)}
	success, err := NewSuccessfulBuildAttempt(inspection, artifact, 1, BuildTriggerAutomatic, "", "worker", now, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewBuildRetryRequest(inspection, success, "retry", "admin", now); err == nil {
		t.Fatal("successful attempt was retryable")
	}
}
