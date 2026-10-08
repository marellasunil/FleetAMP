package lifecycle

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type GitOpsExecutionStatus string

const (
	GitOpsExecutionQueued    GitOpsExecutionStatus = "queued"
	GitOpsExecutionClaimed   GitOpsExecutionStatus = "claimed"
	GitOpsExecutionExecuting GitOpsExecutionStatus = "executing"
	GitOpsExecutionSucceeded GitOpsExecutionStatus = "succeeded"
	GitOpsExecutionFailed    GitOpsExecutionStatus = "failed"
	GitOpsChangeOpen         GitOpsExecutionStatus = "change_open"
	GitOpsChangeMerged       GitOpsExecutionStatus = "change_merged"
	GitOpsChangeClosed       GitOpsExecutionStatus = "change_closed"
)

// GitOpsExecutionRequest is an immutable outbox command for one approved
// preview. Provider adapters consume it later; creating it performs no Git IO.
type GitOpsExecutionRequest struct {
	ID             string    `json:"id"`
	ApprovalID     string    `json:"approval_id"`
	PreviewID      string    `json:"preview_id"`
	PreviewHash    string    `json:"preview_hash"`
	PlanHash       string    `json:"plan_hash"`
	ConnectionID   string    `json:"connection_id"`
	Provider       string    `json:"provider"`
	Mode           string    `json:"mode"`
	RepositoryPath string    `json:"repository_path"`
	Branch         string    `json:"branch"`
	RequestedBy    string    `json:"requested_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// GitOpsExecutionEvent is append-only audit evidence for an execution request.
type GitOpsExecutionEvent struct {
	ID          string                `json:"id"`
	ExecutionID string                `json:"execution_id"`
	Status      GitOpsExecutionStatus `json:"status"`
	Actor       string                `json:"actor"`
	Message     string                `json:"message"`
	Evidence    map[string]string     `json:"evidence,omitempty"`
	CreatedAt   time.Time             `json:"created_at"`
}

func NewGitOpsExecutionRequest(approval *GitOpsPreviewApproval, preview *GitOpsPreview, requestedBy string) (*GitOpsExecutionRequest, *GitOpsExecutionEvent, error) {
	requestedBy = strings.TrimSpace(requestedBy)
	if approval == nil || preview == nil || requestedBy == "" {
		return nil, nil, errors.New("approved preview evidence and requester are required")
	}
	if approval.Status != ApprovalApproved || approval.PreviewID != preview.ID || approval.PreviewHash != preview.PreviewHash || approval.PlanHash != preview.PlanHash || approval.ConnectionID != preview.Connection.ID || approval.RepositoryPath != preview.RepositoryPath || approval.Branch != preview.Connection.Branch {
		return nil, nil, errors.New("approval does not match exact immutable preview evidence")
	}
	id, err := randomGitOpsExecutionID()
	if err != nil {
		return nil, nil, err
	}
	eventID, err := randomGitOpsExecutionID()
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	request := &GitOpsExecutionRequest{ID: id, ApprovalID: approval.ID, PreviewID: preview.ID, PreviewHash: preview.PreviewHash, PlanHash: preview.PlanHash, ConnectionID: preview.Connection.ID, Provider: preview.Connection.Provider, Mode: preview.Connection.Mode, RepositoryPath: preview.RepositoryPath, Branch: preview.Connection.Branch, RequestedBy: requestedBy, CreatedAt: now}
	event := &GitOpsExecutionEvent{ID: eventID, ExecutionID: id, Status: GitOpsExecutionQueued, Actor: requestedBy, Message: "Approved immutable preview queued for provider execution", Evidence: map[string]string{"approval_id": approval.ID, "preview_hash": preview.PreviewHash}, CreatedAt: now}
	return request, event, nil
}

func randomGitOpsExecutionID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func NewGitOpsExecutionEvent(executionID string, status GitOpsExecutionStatus, actor, message string, evidence map[string]string) (*GitOpsExecutionEvent, error) {
	executionID, actor, message = strings.TrimSpace(executionID), strings.TrimSpace(actor), strings.TrimSpace(message)
	if executionID == "" || actor == "" || message == "" {
		return nil, errors.New("execution ID, actor, and message are required")
	}
	switch status {
	case GitOpsExecutionClaimed, GitOpsExecutionExecuting, GitOpsExecutionSucceeded, GitOpsExecutionFailed, GitOpsChangeOpen, GitOpsChangeMerged, GitOpsChangeClosed:
	default:
		return nil, errors.New("invalid execution event status")
	}
	id, err := randomGitOpsExecutionID()
	if err != nil {
		return nil, err
	}
	return &GitOpsExecutionEvent{ID: id, ExecutionID: executionID, Status: status, Actor: actor, Message: message, Evidence: evidence, CreatedAt: time.Now().UTC()}, nil
}

func NewGitOpsExecutionRetryEvent(executionID, actor, reason string, attempt int) (*GitOpsExecutionEvent, error) {
	executionID, actor, reason = strings.TrimSpace(executionID), strings.TrimSpace(actor), strings.TrimSpace(reason)
	if executionID == "" || actor == "" || reason == "" {
		return nil, errors.New("execution ID, retry actor, and retry reason are required")
	}
	if attempt < 2 {
		return nil, errors.New("retry attempt must be at least 2")
	}
	id, err := randomGitOpsExecutionID()
	if err != nil {
		return nil, err
	}
	return &GitOpsExecutionEvent{
		ID:          id,
		ExecutionID: executionID,
		Status:      GitOpsExecutionQueued,
		Actor:       actor,
		Message:     "Failed GitOps execution queued for retry: " + reason,
		Evidence:    map[string]string{"retry": "true", "attempt": fmt.Sprintf("%d", attempt), "reason": reason},
		CreatedAt:   time.Now().UTC(),
	}, nil
}

func CloneGitOpsExecutionRequest(v *GitOpsExecutionRequest) *GitOpsExecutionRequest {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func CloneGitOpsExecutionEvent(v *GitOpsExecutionEvent) *GitOpsExecutionEvent {
	if v == nil {
		return nil
	}
	c := *v
	c.Evidence = map[string]string{}
	for k, value := range v.Evidence {
		c.Evidence[k] = value
	}
	return &c
}
