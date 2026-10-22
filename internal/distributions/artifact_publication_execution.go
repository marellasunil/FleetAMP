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
	ArtifactPublicationTriggerAutomatic = "automatic"
	ArtifactPublicationTriggerManual    = "manual"
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
	AttemptNumber                                                                  int
	Trigger, RetryRequestID                                                        string
	StartedAt, CompletedAt                                                         time.Time
}

func NewSuccessfulArtifactPublicationExecution(publication *ArtifactPublication, preflight *ArtifactPublicationPreflight, result *RegistryPublishResult, attemptNumber int, trigger, retryRequestID, worker string, startedAt, completedAt time.Time) (*ArtifactPublicationExecution, error) {
	if result == nil || result.ArtifactSHA256 != publication.ArtifactSHA256 || result.DestinationRef != publication.DestinationRef {
		return nil, errors.New("registry result does not match exact publication evidence")
	}
	return newArtifactPublicationExecution(publication, preflight, ArtifactPublicationSucceeded, "", "", attemptNumber, trigger, retryRequestID, worker, startedAt, completedAt)
}

func NewFailedArtifactPublicationExecution(publication *ArtifactPublication, preflight *ArtifactPublicationPreflight, failureCode, diagnostic string, attemptNumber int, trigger, retryRequestID, worker string, startedAt, completedAt time.Time) (*ArtifactPublicationExecution, error) {
	if failureCode != ArtifactPublicationFailurePublisher && failureCode != ArtifactPublicationFailureTimeout {
		return nil, errors.New("unsupported publication failure code")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(diagnostic)))
	return newArtifactPublicationExecution(publication, preflight, ArtifactPublicationFailed, failureCode, hex.EncodeToString(sum[:]), attemptNumber, trigger, retryRequestID, worker, startedAt, completedAt)
}

func newArtifactPublicationExecution(publication *ArtifactPublication, preflight *ArtifactPublicationPreflight, status, failureCode, failureDigest string, attemptNumber int, trigger, retryRequestID, worker string, startedAt, completedAt time.Time) (*ArtifactPublicationExecution, error) {
	if publication == nil || preflight == nil || preflight.PublicationID != publication.ID || preflight.PublicationEvidenceHash != publication.EvidenceHash || preflight.Status != ArtifactPublicationPreflightPassed {
		return nil, errors.New("matching successful publication preflight is required")
	}
	worker = strings.TrimSpace(worker)
	if worker == "" || completedAt.Before(startedAt) {
		return nil, errors.New("publication worker and valid timestamps are required")
	}
	trigger, retryRequestID = strings.TrimSpace(trigger), strings.TrimSpace(retryRequestID)
	if attemptNumber <= 0 || (trigger != ArtifactPublicationTriggerAutomatic && trigger != ArtifactPublicationTriggerManual) {
		return nil, errors.New("valid publication attempt number and trigger are required")
	}
	if (trigger == ArtifactPublicationTriggerAutomatic && (attemptNumber != 1 || retryRequestID != "")) || (trigger == ArtifactPublicationTriggerManual && (attemptNumber <= 1 || retryRequestID == "")) {
		return nil, errors.New("publication retry evidence is inconsistent")
	}
	payload := struct {
		PublicationID, PublicationEvidenceHash, PreflightID, PreflightEvidenceHash string
		ArtifactID, ArtifactSHA256, DestinationRef, CredentialRef                  string
		Status, FailureCode, FailureDigest, Trigger, RetryRequestID, Worker        string
		AttemptNumber                                                              int
		StartedAt, CompletedAt                                                     time.Time
	}{publication.ID, publication.EvidenceHash, preflight.ID, preflight.EvidenceHash, publication.ArtifactID, publication.ArtifactSHA256, publication.DestinationRef, publication.CredentialRef, status, failureCode, failureDigest, trigger, retryRequestID, worker, attemptNumber, startedAt.UTC(), completedAt.UTC()}
	canonical, _ := json.Marshal(payload)
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &ArtifactPublicationExecution{ID: "artifact-publication-execution-" + evidence[:16], PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, PreflightID: preflight.ID, PreflightEvidenceHash: preflight.EvidenceHash, ArtifactID: publication.ArtifactID, ArtifactSHA256: publication.ArtifactSHA256, DestinationRef: publication.DestinationRef, CredentialRef: publication.CredentialRef, Status: status, FailureCode: failureCode, FailureDigest: failureDigest, AttemptNumber: attemptNumber, Trigger: trigger, RetryRequestID: retryRequestID, WorkerID: worker, EvidenceHash: evidence, StartedAt: startedAt.UTC(), CompletedAt: completedAt.UTC()}, nil
}

type ArtifactPublicationRetryRequest struct {
	ID, PublicationID, PublicationEvidenceHash, PreflightID, PreflightEvidenceHash string
	PreviousExecutionID, PreviousExecutionEvidenceHash, Reason, RequestedBy        string
	EvidenceHash                                                                   string
	RequestedAt                                                                    time.Time
}

func NewArtifactPublicationRetryRequest(publication *ArtifactPublication, preflight *ArtifactPublicationPreflight, previous *ArtifactPublicationExecution, reason, actor string, now time.Time) (*ArtifactPublicationRetryRequest, error) {
	if publication == nil || preflight == nil || previous == nil || previous.Status != ArtifactPublicationFailed || previous.PublicationID != publication.ID || previous.PublicationEvidenceHash != publication.EvidenceHash || previous.PreflightID != preflight.ID || previous.PreflightEvidenceHash != preflight.EvidenceHash {
		return nil, errors.New("matching failed publication execution is required")
	}
	reason, actor = strings.TrimSpace(reason), strings.TrimSpace(actor)
	if reason == "" || actor == "" {
		return nil, errors.New("retry reason and requester are required")
	}
	if len(reason) > 500 {
		return nil, errors.New("retry reason must not exceed 500 characters")
	}
	payload := struct {
		PublicationID, PublicationEvidenceHash, PreflightID, PreflightEvidenceHash string
		PreviousExecutionID, PreviousExecutionEvidenceHash, Reason, Actor          string
		At                                                                         time.Time
	}{publication.ID, publication.EvidenceHash, preflight.ID, preflight.EvidenceHash, previous.ID, previous.EvidenceHash, reason, actor, now.UTC()}
	canonical, _ := json.Marshal(payload)
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &ArtifactPublicationRetryRequest{ID: "artifact-publication-retry-" + evidence[:16], PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, PreflightID: preflight.ID, PreflightEvidenceHash: preflight.EvidenceHash, PreviousExecutionID: previous.ID, PreviousExecutionEvidenceHash: previous.EvidenceHash, Reason: reason, RequestedBy: actor, EvidenceHash: evidence, RequestedAt: now.UTC()}, nil
}
