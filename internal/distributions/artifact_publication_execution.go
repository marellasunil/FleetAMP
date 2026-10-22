package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	ArtifactPublicationSucceeded        = "succeeded"
	ArtifactPublicationFailed           = "failed"
	ArtifactPublicationFailurePublisher = "publisher-error"
	ArtifactPublicationFailureTimeout   = "timeout"
)

type RegistryPublishResult struct {
	ArtifactSHA256, DestinationRef string
}

type ArtifactRegistryPublisher interface {
	Publish(context.Context, *ArtifactPublicationPreflight) (*RegistryPublishResult, error)
}

// ArtifactPublicationExecution is an immutable receipt for exactly one leased
// publication attempt. Failure details are deliberately reduced to a safe code
// and diagnostic fingerprint so registry credentials cannot enter the database.
type ArtifactPublicationExecution struct {
	ID, PublicationID, PublicationEvidenceHash, PreflightID, PreflightEvidenceHash string
	ArtifactID, ArtifactSHA256, DestinationRef, CredentialRef                      string
	Status, FailureCode, FailureDigest, WorkerID, EvidenceHash                     string
	StartedAt, CompletedAt                                                         time.Time
}

func NewSuccessfulArtifactPublicationExecution(publication *ArtifactPublication, preflight *ArtifactPublicationPreflight, result *RegistryPublishResult, worker string, startedAt, completedAt time.Time) (*ArtifactPublicationExecution, error) {
	if result == nil || result.ArtifactSHA256 != publication.ArtifactSHA256 || result.DestinationRef != publication.DestinationRef {
		return nil, errors.New("registry result does not match exact publication evidence")
	}
	return newArtifactPublicationExecution(publication, preflight, ArtifactPublicationSucceeded, "", "", worker, startedAt, completedAt)
}

func NewFailedArtifactPublicationExecution(publication *ArtifactPublication, preflight *ArtifactPublicationPreflight, failureCode, diagnostic, worker string, startedAt, completedAt time.Time) (*ArtifactPublicationExecution, error) {
	if failureCode != ArtifactPublicationFailurePublisher && failureCode != ArtifactPublicationFailureTimeout {
		return nil, errors.New("unsupported publication failure code")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(diagnostic)))
	return newArtifactPublicationExecution(publication, preflight, ArtifactPublicationFailed, failureCode, hex.EncodeToString(sum[:]), worker, startedAt, completedAt)
}

func newArtifactPublicationExecution(publication *ArtifactPublication, preflight *ArtifactPublicationPreflight, status, failureCode, failureDigest, worker string, startedAt, completedAt time.Time) (*ArtifactPublicationExecution, error) {
	if publication == nil || preflight == nil || preflight.PublicationID != publication.ID || preflight.PublicationEvidenceHash != publication.EvidenceHash || preflight.Status != ArtifactPublicationPreflightPassed {
		return nil, errors.New("matching successful publication preflight is required")
	}
	worker = strings.TrimSpace(worker)
	if worker == "" || completedAt.Before(startedAt) {
		return nil, errors.New("publication worker and valid timestamps are required")
	}
	payload := struct {
		PublicationID, PublicationEvidenceHash, PreflightID, PreflightEvidenceHash string
		ArtifactID, ArtifactSHA256, DestinationRef, CredentialRef                  string
		Status, FailureCode, FailureDigest, Worker                                 string
		StartedAt, CompletedAt                                                     time.Time
	}{publication.ID, publication.EvidenceHash, preflight.ID, preflight.EvidenceHash, publication.ArtifactID, publication.ArtifactSHA256, publication.DestinationRef, publication.CredentialRef, status, failureCode, failureDigest, worker, startedAt.UTC(), completedAt.UTC()}
	canonical, _ := json.Marshal(payload)
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &ArtifactPublicationExecution{ID: "artifact-publication-execution-" + evidence[:16], PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, PreflightID: preflight.ID, PreflightEvidenceHash: preflight.EvidenceHash, ArtifactID: publication.ArtifactID, ArtifactSHA256: publication.ArtifactSHA256, DestinationRef: publication.DestinationRef, CredentialRef: publication.CredentialRef, Status: status, FailureCode: failureCode, FailureDigest: failureDigest, WorkerID: worker, EvidenceHash: evidence, StartedAt: startedAt.UTC(), CompletedAt: completedAt.UTC()}, nil
}
