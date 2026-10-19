package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	BuildAttemptSucceeded = "succeeded"
	BuildAttemptFailed    = "failed"
	BuildTriggerAutomatic = "automatic"
	BuildTriggerManual    = "manual"
	BuildFailureTimeout   = "timeout"
	BuildFailureRunner    = "runner-error"
	BuildFailureArtifact  = "artifact-validation"
)

// BuildAttempt is an immutable receipt for one completed sandboxed OCB run.
type BuildAttempt struct {
	ID, InspectionID, ExecutionID, BuildPlanHash, ManifestSHA256 string
	AttemptNumber                                                int
	Trigger, RetryRequestID, Status                              string
	FailureCode, FailureMessage, ArtifactID, ArtifactSHA256      string
	WorkerID, EvidenceHash                                       string
	StartedAt, CompletedAt, NextRetryAt                          time.Time
}

func NewSuccessfulBuildAttempt(inspection *BuildSourceInspection, artifact *BuildArtifact, number int, trigger, retryRequestID, workerID string, startedAt, completedAt time.Time) (*BuildAttempt, error) {
	if inspection == nil || artifact == nil || artifact.InspectionID != inspection.ID {
		return nil, errors.New("matching source inspection and artifact are required")
	}
	return newBuildAttempt(inspection, number, trigger, retryRequestID, BuildAttemptSucceeded, "", "", artifact.ID, artifact.ArtifactSHA256, workerID, startedAt, completedAt, time.Time{})
}

func NewFailedBuildAttempt(inspection *BuildSourceInspection, number int, trigger, retryRequestID, failureCode, failureMessage, workerID string, startedAt, completedAt, nextRetryAt time.Time) (*BuildAttempt, error) {
	if failureCode != BuildFailureTimeout && failureCode != BuildFailureRunner && failureCode != BuildFailureArtifact {
		return nil, errors.New("unsupported build failure code")
	}
	return newBuildAttempt(inspection, number, trigger, retryRequestID, BuildAttemptFailed, failureCode, sanitizeFailureMessage(failureMessage), "", "", workerID, startedAt, completedAt, nextRetryAt)
}

func newBuildAttempt(inspection *BuildSourceInspection, number int, trigger, retryRequestID, status, failureCode, failureMessage, artifactID, artifactSHA, workerID string, startedAt, completedAt, nextRetryAt time.Time) (*BuildAttempt, error) {
	if inspection == nil || inspection.Status != BuildSourceInspected {
		return nil, errors.New("inspected source is required")
	}
	if number < 1 || (trigger != BuildTriggerAutomatic && trigger != BuildTriggerManual) {
		return nil, errors.New("valid attempt number and trigger are required")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" || completedAt.Before(startedAt) {
		return nil, errors.New("worker and valid attempt timestamps are required")
	}
	if trigger == BuildTriggerManual && strings.TrimSpace(retryRequestID) == "" {
		return nil, errors.New("manual attempt requires an immutable retry request")
	}
	if status == BuildAttemptSucceeded && (artifactID == "" || len(artifactSHA) != 64 || !isHex(artifactSHA)) {
		return nil, errors.New("successful attempt requires valid artifact evidence")
	}
	payload := struct {
		InspectionID, ExecutionID, PlanHash, ManifestSHA               string
		Number                                                         int
		Trigger, RetryRequestID, Status                                string
		FailureCode, FailureMessage, ArtifactID, ArtifactSHA, WorkerID string
		StartedAt, CompletedAt, NextRetryAt                            time.Time
	}{inspection.ID, inspection.ExecutionID, inspection.BuildPlanHash, inspection.ManifestSHA256, number, trigger, strings.TrimSpace(retryRequestID), status, failureCode, failureMessage, artifactID, artifactSHA, workerID, startedAt.UTC(), completedAt.UTC(), nextRetryAt.UTC()}
	canonical, _ := json.Marshal(payload)
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &BuildAttempt{ID: "build-attempt-" + evidence[:16], InspectionID: inspection.ID, ExecutionID: inspection.ExecutionID, BuildPlanHash: inspection.BuildPlanHash, ManifestSHA256: inspection.ManifestSHA256, AttemptNumber: number, Trigger: trigger, RetryRequestID: strings.TrimSpace(retryRequestID), Status: status, FailureCode: failureCode, FailureMessage: failureMessage, ArtifactID: artifactID, ArtifactSHA256: artifactSHA, WorkerID: workerID, EvidenceHash: evidence, StartedAt: startedAt.UTC(), CompletedAt: completedAt.UTC(), NextRetryAt: nextRetryAt.UTC()}, nil
}

func sanitizeFailureMessage(value string) string {
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	value = strings.Join(strings.Fields(value), " ")
	value = regexp.MustCompile(`(?i)\b(token|password|authorization|secret)=\S+`).ReplaceAllString(value, "$1=[REDACTED]")
	value = regexp.MustCompile(`(?i)\bbearer\s+\S+`).ReplaceAllString(value, "Bearer [REDACTED]")
	if len(value) > 2048 {
		value = value[:2048]
	}
	if value == "" {
		return "sandboxed OCB build failed without diagnostic output"
	}
	return value
}

// BuildRetryRequest is immutable authorization for exactly one additional run.
type BuildRetryRequest struct {
	ID, InspectionID, PreviousAttemptID, Reason, RequestedBy, EvidenceHash string
	RequestedAt                                                            time.Time
}

func NewBuildRetryRequest(inspection *BuildSourceInspection, previous *BuildAttempt, reason, actor string, now time.Time) (*BuildRetryRequest, error) {
	if inspection == nil || previous == nil || previous.InspectionID != inspection.ID || previous.Status != BuildAttemptFailed {
		return nil, errors.New("latest failed attempt is required")
	}
	reason, actor = strings.TrimSpace(reason), strings.TrimSpace(actor)
	if reason == "" || actor == "" {
		return nil, errors.New("retry reason and requester are required")
	}
	if len(reason) > 500 {
		return nil, errors.New("retry reason must not exceed 500 characters")
	}
	canonical, _ := json.Marshal(struct {
		InspectionID, PreviousAttemptID, Reason, Actor string
		At                                             time.Time
	}{inspection.ID, previous.ID, reason, actor, now.UTC()})
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &BuildRetryRequest{ID: "build-retry-" + evidence[:16], InspectionID: inspection.ID, PreviousAttemptID: previous.ID, Reason: reason, RequestedBy: actor, EvidenceHash: evidence, RequestedAt: now.UTC()}, nil
}
