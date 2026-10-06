package lifecycle

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending_approval"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
	ApprovalSentBack ApprovalStatus = "sent_back"
)

type Approval struct {
	ID                 string         `json:"id"`
	RequestID          string         `json:"request_id"`
	RequestSpecHash    string         `json:"request_spec_hash"`
	ValidationID       string         `json:"validation_id"`
	ValidationHash     string         `json:"validation_hash"`
	GroupID            string         `json:"group_id"`
	GroupName          string         `json:"group_name"`
	Operation          Operation      `json:"operation"`
	ComponentType      string         `json:"component_type"`
	TargetCount        int            `json:"target_count"`
	RequestedBy        string         `json:"requested_by"`
	AssignedReviewer   string         `json:"assigned_reviewer"`
	SubmissionComment  string         `json:"submission_comment"`
	Status             ApprovalStatus `json:"status"`
	ReviewedBy         string         `json:"reviewed_by,omitempty"`
	ReviewComment      string         `json:"review_comment,omitempty"`
	ReviewedAt         *time.Time     `json:"reviewed_at,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
}

func NewApproval(request *Request, validation *Validation, requestedBy, reviewer, comment string) (*Approval, error) {
	if request == nil || validation == nil || validation.RequestID != request.ID || validation.RequestSpecHash != request.SpecHash {
		return nil, errors.New("proposal and validation snapshot do not match")
	}
	if validation.Status != ValidationCompatible {
		return nil, errors.New("only a compatible validation snapshot can be submitted for approval")
	}
	requestedBy, reviewer, comment = strings.TrimSpace(requestedBy), strings.TrimSpace(reviewer), strings.TrimSpace(comment)
	if requestedBy == "" || reviewer == "" || comment == "" || strings.EqualFold(requestedBy, reviewer) {
		return nil, errors.New("requester, a different reviewer, and submission comment are required")
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil { return nil, err }
	return &Approval{ID: hex.EncodeToString(raw), RequestID: request.ID, RequestSpecHash: request.SpecHash,
		ValidationID: validation.ID, ValidationHash: validation.ResultHash, GroupID: validation.GroupID, GroupName: validation.GroupName,
		Operation: request.Spec.Operation, ComponentType: string(request.Spec.ComponentType), TargetCount: len(validation.Targets),
		RequestedBy: requestedBy, AssignedReviewer: reviewer, SubmissionComment: comment, Status: ApprovalPending, CreatedAt: time.Now().UTC()}, nil
}

func CloneApproval(value *Approval) *Approval {
	if value == nil { return nil }
	copy := *value
	if value.ReviewedAt != nil { reviewed := *value.ReviewedAt; copy.ReviewedAt = &reviewed }
	return &copy
}
