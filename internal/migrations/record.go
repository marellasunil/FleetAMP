// Package migrations models immutable audit metadata for configurations created
// through FleetAMP's staged migration workspace.
package migrations

import "time"

// Record links one saved group configuration version to the migration decision
// that produced it. Records are append-only and contain no secret values.
type Record struct {
	ID              string    `json:"id"`
	ConfigurationID string    `json:"configuration_id"`
	GroupID         string    `json:"group_id"`
	GroupName       string    `json:"group_name"`
	Name            string    `json:"name"`
	Version         string    `json:"version"`
	Source          string    `json:"source"`
	AgentUID        string    `json:"agent_uid,omitempty"`
	PatternID       string    `json:"pattern_id"`
	ContentHash     string    `json:"content_hash"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}
