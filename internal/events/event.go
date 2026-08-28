// FleetAMP managed-agent lifecycle event model.
//
// Purpose:
//   Provides append-only historical events for connection, disconnection,
//   retirement, and health changes. These events power time-range fleet views
//   and form the foundation for future audit/history features.
//
// Dependencies:
//   Go time package only. Event persistence is defined separately in storage.

package events

import "time"

type Type string

const (
	Connected     Type = "connected"
	Disconnected  Type = "disconnected"
	Retired       Type = "retired"
	HealthChanged Type = "health_changed"
)

type AgentEvent struct {
	AgentInstanceUID string            `json:"agent_instance_uid"`
	AgentName        string            `json:"agent_name,omitempty"`
	Type             Type              `json:"type"`
	Timestamp        time.Time         `json:"timestamp"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}
