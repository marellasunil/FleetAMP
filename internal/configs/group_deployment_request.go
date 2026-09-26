package configs

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type GroupDeploymentRequestStatus string

const (
	GroupDeploymentPendingApproval GroupDeploymentRequestStatus = "pending_approval"
	GroupDeploymentDeploying       GroupDeploymentRequestStatus = "deploying"
	GroupDeploymentCompleted       GroupDeploymentRequestStatus = "completed"
	GroupDeploymentRejected        GroupDeploymentRequestStatus = "rejected"
	GroupDeploymentFailed          GroupDeploymentRequestStatus = "failed"
)

type GroupDeploymentTarget struct {
	AgentInstanceUID string `json:"agent_instance_uid"`
	AgentName        string `json:"agent_name,omitempty"`
	Readiness        string `json:"readiness"`
	Eligible         bool   `json:"eligible"`
}

// GroupDeploymentRequest is an immutable approval request. Targets are
// snapshotted at request time so reviewers approve a precise, auditable scope.
type GroupDeploymentRequest struct {
	ID                    string                       `json:"id"`
	GroupID               string                       `json:"group_id"`
	GroupName             string                       `json:"group_name"`
	GroupSelector         map[string]string            `json:"group_selector"`
	ConfigurationID       string                       `json:"configuration_id"`
	ConfigurationName     string                       `json:"configuration_name"`
	ConfigurationVersion  string                       `json:"configuration_version"`
	ConfigurationHash     string                       `json:"configuration_hash"`
	BaseConfigurationID   string                       `json:"base_configuration_id,omitempty"`
	BaseConfigurationHash string                       `json:"base_configuration_hash,omitempty"`
	Targets               []GroupDeploymentTarget      `json:"targets"`
	RequestedBy           string                       `json:"requested_by"`
	ReviewedBy            string                       `json:"reviewed_by,omitempty"`
	ReviewComment         string                       `json:"review_comment,omitempty"`
	ReviewedAt            *time.Time                   `json:"reviewed_at,omitempty"`
	Status                GroupDeploymentRequestStatus `json:"status"`
	CreatedAt             time.Time                    `json:"created_at"`
}

func NewGroupDeploymentRequest(groupID, groupName string, selector map[string]string, configuration *Configuration, targets []GroupDeploymentTarget, requestedBy string) (*GroupDeploymentRequest, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	selectorCopy := make(map[string]string, len(selector))
	for key, value := range selector {
		selectorCopy[key] = value
	}
	return &GroupDeploymentRequest{
		ID: hex.EncodeToString(raw), GroupID: groupID, GroupName: groupName,
		GroupSelector: selectorCopy, ConfigurationID: configuration.ID,
		ConfigurationName: configuration.Name, ConfigurationVersion: configuration.Version,
		ConfigurationHash: configuration.Hash, Targets: append([]GroupDeploymentTarget(nil), targets...),
		RequestedBy: requestedBy, Status: GroupDeploymentPendingApproval, CreatedAt: time.Now().UTC(),
	}, nil
}
