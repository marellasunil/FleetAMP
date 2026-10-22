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
	receipt, err := NewSuccessfulArtifactPublicationExecution(publication, preflight, result, "worker-1", start, start.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != ArtifactPublicationSucceeded || receipt.PublicationEvidenceHash != publication.EvidenceHash || receipt.PreflightEvidenceHash != preflight.EvidenceHash || receipt.FailureDigest != "" {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	result.DestinationRef = "oci://registry.example.test/other:v1"
	if _, err := NewSuccessfulArtifactPublicationExecution(publication, preflight, result, "worker-1", start, start.Add(time.Second)); err == nil {
		t.Fatal("expected mismatched registry result to fail")
	}
}

func TestFailedArtifactPublicationExecutionStoresFingerprintOnly(t *testing.T) {
	publication, preflight := publicationExecutionFixture()
	now := time.Now().UTC()
	receipt, err := NewFailedArtifactPublicationExecution(publication, preflight, ArtifactPublicationFailurePublisher, "Authorization: Bearer top-secret", "worker-1", now, now)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != ArtifactPublicationFailed || len(receipt.FailureDigest) != 64 || strings.Contains(receipt.FailureDigest, "top-secret") {
		t.Fatalf("unsafe failure receipt: %#v", receipt)
	}
}
