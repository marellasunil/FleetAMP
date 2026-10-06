package lifecycle

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// GitOpsPreviewApproval is the four-eyes decision for one exact immutable
// repository preview. A different preview hash always requires a new record.
type GitOpsPreviewApproval struct {
	ID                string         `json:"id"`
	PreviewID         string         `json:"preview_id"`
	PreviewHash       string         `json:"preview_hash"`
	PlanHash          string         `json:"plan_hash"`
	ConnectionID      string         `json:"connection_id"`
	RepositoryPath    string         `json:"repository_path"`
	Branch            string         `json:"branch"`
	SubmittedBy       string         `json:"submitted_by"`
	AssignedReviewer  string         `json:"assigned_reviewer"`
	SubmissionComment string         `json:"submission_comment"`
	Status            ApprovalStatus `json:"status"`
	ReviewedBy        string         `json:"reviewed_by,omitempty"`
	ReviewComment     string         `json:"review_comment,omitempty"`
	ReviewedAt        *time.Time     `json:"reviewed_at,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
}

func NewGitOpsPreviewApproval(preview *GitOpsPreview, submittedBy, reviewer, comment string) (*GitOpsPreviewApproval, error) {
	if preview == nil || preview.ID == "" || preview.PreviewHash == "" || preview.PlanHash == "" || preview.Connection.ID == "" || preview.RepositoryPath == "" || preview.Connection.Branch == "" {
		return nil, errors.New("complete immutable GitOps preview evidence is required")
	}
	submittedBy, reviewer, comment = strings.TrimSpace(submittedBy), strings.TrimSpace(reviewer), strings.TrimSpace(comment)
	if submittedBy == "" || reviewer == "" || comment == "" || strings.EqualFold(submittedBy, reviewer) {
		return nil, errors.New("submitter, a different reviewer, and submission comment are required")
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	return &GitOpsPreviewApproval{ID: hex.EncodeToString(raw), PreviewID: preview.ID, PreviewHash: preview.PreviewHash, PlanHash: preview.PlanHash, ConnectionID: preview.Connection.ID, RepositoryPath: preview.RepositoryPath, Branch: preview.Connection.Branch, SubmittedBy: submittedBy, AssignedReviewer: reviewer, SubmissionComment: comment, Status: ApprovalPending, CreatedAt: time.Now().UTC()}, nil
}

func CloneGitOpsPreviewApproval(value *GitOpsPreviewApproval) *GitOpsPreviewApproval {
	if value == nil {
		return nil
	}
	copy := *value
	if value.ReviewedAt != nil {
		reviewed := *value.ReviewedAt
		copy.ReviewedAt = &reviewed
	}
	return &copy
}
