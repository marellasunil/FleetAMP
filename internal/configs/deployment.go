// Append-only FleetAMP configuration deployment history model.
//
// Purpose:
//
//	Records every configuration delivery attempt independently from the current
//	desired-state assignment. Re-deploying or rolling back to the same immutable
//	configuration therefore creates a new auditable deployment record.
package configs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var ErrDeploymentInProgress = errors.New("configuration deployment already in progress for agent")

type DeploymentAction string

const (
	DeploymentActionDeploy   DeploymentAction = "deploy"
	DeploymentActionRollback DeploymentAction = "rollback"
)

type Deployment struct {
	ID                   string           `json:"id"`
	AgentInstanceUID     string           `json:"agent_instance_uid"`
	ConfigurationID      string           `json:"configuration_id"`
	ConfigurationName    string           `json:"configuration_name"`
	ConfigurationVersion string           `json:"configuration_version"`
	ConfigurationHash    string           `json:"configuration_hash"`
	Action               DeploymentAction `json:"action"`
	Status               DeliveryStatus   `json:"status"`
	Error                string           `json:"error,omitempty"`
	CreatedAt            time.Time        `json:"created_at"`
	SentAt               *time.Time       `json:"sent_at,omitempty"`
	ApplyingAt           *time.Time       `json:"applying_at,omitempty"`
	AppliedAt            *time.Time       `json:"applied_at,omitempty"`
	FailedAt             *time.Time       `json:"failed_at,omitempty"`
	UpdatedAt            time.Time        `json:"updated_at"`
}

// NewDeployment creates a pending delivery-history record for a configuration action against one agent.
func NewDeployment(agentUID string, configuration *Configuration, action DeploymentAction) (*Deployment, error) {
	if configuration == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	id, err := newDeploymentID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	return &Deployment{
		ID: id, AgentInstanceUID: agentUID, ConfigurationID: configuration.ID,
		ConfigurationName: configuration.Name, ConfigurationVersion: configuration.Version,
		ConfigurationHash: configuration.Hash, Action: action, Status: DeliveryPending,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// newDeploymentID generates a random identifier suitable for persistent deployment records.
func newDeploymentID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate deployment id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// SetStatus advances a deployment and records timing and error details appropriate to the new state.
func (d *Deployment) SetStatus(status DeliveryStatus, errText string, at time.Time) {
	at = at.UTC()
	d.Status, d.Error, d.UpdatedAt = status, errText, at
	switch status {
	case DeliverySent:
		d.SentAt = &at
	case DeliveryApplying:
		d.ApplyingAt = &at
	case DeliveryApplied:
		d.AppliedAt = &at
	case DeliveryFailed, DeliveryUnsupported:
		d.FailedAt = &at
	}
}
