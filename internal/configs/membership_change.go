package configs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type MembershipOperation string
type RemovalPolicy string
type MembershipStatus string

const (
	MembershipAssign    MembershipOperation = "assign"
	MembershipRemove    MembershipOperation = "remove"
	MembershipMove      MembershipOperation = "move"
	RemovalKeep         RemovalPolicy       = "keep_current"
	RemovalRestore      RemovalPolicy       = "restore_pre_group"
	MembershipPending   MembershipStatus    = "pending_approval"
	MembershipExecuting MembershipStatus    = "executing"
	MembershipCompleted MembershipStatus    = "completed"
	MembershipRejected  MembershipStatus    = "rejected"
	MembershipFailed    MembershipStatus    = "failed"
)

type MembershipTarget struct {
	AgentInstanceUID        string            `json:"agent_instance_uid"`
	AgentName               string            `json:"agent_name"`
	PreviousGroupFields     map[string]string `json:"previous_group_fields,omitempty"`
	PreviousConfigurationID string            `json:"previous_configuration_id,omitempty"`
}

type MembershipChange struct {
	ID                    string              `json:"id"`
	Operation             MembershipOperation `json:"operation"`
	SourceGroupID         string              `json:"source_group_id,omitempty"`
	SourceGroupName       string              `json:"source_group_name,omitempty"`
	TargetGroupID         string              `json:"target_group_id,omitempty"`
	TargetGroupName       string              `json:"target_group_name,omitempty"`
	RemovalPolicy         RemovalPolicy       `json:"removal_policy,omitempty"`
	TargetConfigurationID string              `json:"target_configuration_id,omitempty"`
	Targets               []MembershipTarget  `json:"targets"`
	RequestedBy           string              `json:"requested_by"`
	AssignedReviewer      string              `json:"assigned_reviewer"`
	Reason                string              `json:"reason"`
	Status                MembershipStatus    `json:"status"`
	ReviewedBy            string              `json:"reviewed_by,omitempty"`
	ReviewComment         string              `json:"review_comment,omitempty"`
	ReviewedAt            *time.Time          `json:"reviewed_at,omitempty"`
	Error                 string              `json:"error,omitempty"`
	CreatedAt             time.Time           `json:"created_at"`
}

func NewMembershipChange(value MembershipChange) (*MembershipChange, error) {
	if value.Operation != MembershipAssign && value.Operation != MembershipRemove && value.Operation != MembershipMove {
		return nil, errors.New("invalid membership operation")
	}
	if len(value.Targets) == 0 || strings.TrimSpace(value.RequestedBy) == "" || strings.TrimSpace(value.AssignedReviewer) == "" || strings.TrimSpace(value.Reason) == "" {
		return nil, errors.New("targets, requester, reviewer and reason are required")
	}
	if value.Operation == MembershipAssign && value.TargetGroupID == "" {
		return nil, errors.New("target group is required")
	}
	if value.Operation == MembershipRemove && value.SourceGroupID == "" {
		return nil, errors.New("source group is required")
	}
	if value.Operation == MembershipMove && (value.SourceGroupID == "" || value.TargetGroupID == "" || value.SourceGroupID == value.TargetGroupID) {
		return nil, errors.New("different source and target groups are required")
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	value.ID, value.Status, value.CreatedAt = hex.EncodeToString(raw), MembershipPending, time.Now().UTC()
	value.Targets = append([]MembershipTarget(nil), value.Targets...)
	return &value, nil
}
