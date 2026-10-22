package distributions

import (
	"strings"
	"testing"
	"time"
)

func publicationExecutionFixture() (*ArtifactPublication, *ArtifactPublicationPreflight) {
	publication := &ArtifactPublication{ID: "publication-1", ArtifactID: "artifact-1", ArtifactSHA256: strings.Repeat("a", 64), DestinationRef: "oci://registry.example.test/fleetamp/collector:v1", CredentialRef: "secret://registry", Status: ArtifactPublicationQueued, EvidenceHash: strings.Repeat("b", 64)}
	preflight := &ArtifactPublicationPreflight{ID: "preflight-1", PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, ArtifactID: publication.ArtifactID, ArtifactSHA256: publication.ArtifactSHA256, DestinationRef: publication.DestinationRef, CredentialRef: publication.CredentialRef, Status: ArtifactPublicationPreflightPassed, EvidenceHash: strings.Repeat("c", 64)}
	return publication, preflight
}

func TestArtifactPublicationExecutionBindsExactEvidence(t *testing.T) {
	publication, preflight := publicationExecutionFixture()
	start := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	result := &RegistryPublishResult{ArtifactSHA256: publication.ArtifactSHA256, DestinationRef: publication.DestinationRef}
	receipt, err := NewSuccessfulArtifactPublicationExecution(publication, preflight, result, 1, ArtifactPublicationTriggerAutomatic, "", "worker-1", start, start.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != ArtifactPublicationSucceeded || receipt.PublicationEvidenceHash != publication.EvidenceHash || receipt.PreflightEvidenceHash != preflight.EvidenceHash || receipt.FailureDigest != "" {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	result.DestinationRef = "oci://registry.example.test/other:v1"
	if _, err := NewSuccessfulArtifactPublicationExecution(publication, preflight, result, 1, ArtifactPublicationTriggerAutomatic, "", "worker-1", start, start.Add(time.Second)); err == nil {
		t.Fatal("expected mismatched registry result to fail")
	}
}

func TestFailedArtifactPublicationExecutionStoresFingerprintOnly(t *testing.T) {
	publication, preflight := publicationExecutionFixture()
	now := time.Now().UTC()
	receipt, err := NewFailedArtifactPublicationExecution(publication, preflight, ArtifactPublicationFailurePublisher, "Authorization: Bearer top-secret", 1, ArtifactPublicationTriggerAutomatic, "", "worker-1", now, now)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != ArtifactPublicationFailed || len(receipt.FailureDigest) != 64 || strings.Contains(receipt.FailureDigest, "top-secret") {
		t.Fatalf("unsafe failure receipt: %#v", receipt)
	}
}

func TestArtifactPublicationRetryBindsLatestFailureEvidence(t *testing.T) {
	publication, preflight := publicationExecutionFixture()
	now := time.Now().UTC()
	failed, err := NewFailedArtifactPublicationExecution(publication, preflight, ArtifactPublicationFailurePublisher, "registry unavailable", 1, ArtifactPublicationTriggerAutomatic, "", "worker-1", now, now)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := NewArtifactPublicationRetryRequest(publication, preflight, failed, "administrator verified registry recovery", "admin", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if retry.PreviousExecutionEvidenceHash != failed.EvidenceHash || retry.PublicationEvidenceHash != publication.EvidenceHash || retry.PreflightEvidenceHash != preflight.EvidenceHash {
		t.Fatalf("retry evidence drifted: %#v", retry)
	}
	result := &RegistryPublishResult{ArtifactSHA256: publication.ArtifactSHA256, DestinationRef: publication.DestinationRef}
	second, err := NewSuccessfulArtifactPublicationExecution(publication, preflight, result, 2, ArtifactPublicationTriggerManual, retry.ID, "worker-1", now.Add(2*time.Second), now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if second.AttemptNumber != 2 || second.Trigger != ArtifactPublicationTriggerManual || second.RetryRequestID != retry.ID {
		t.Fatalf("retry receipt not bound: %#v", second)
	}
	if _, err := NewSuccessfulArtifactPublicationExecution(publication, preflight, result, 2, ArtifactPublicationTriggerManual, "", "worker-1", now, now); err == nil {
		t.Fatal("manual attempt accepted without retry authorization")
	}
}
