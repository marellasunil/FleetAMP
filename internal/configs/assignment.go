// Per-agent configuration assignment and delivery-state model.
//
// Purpose:
//   Correlates one FleetAMP configuration artifact with one managed agent and
//   records the control-plane lifecycle: pending, sent, applying, applied,
//   failed, or unsupported.
//
// Dependencies:
//   Go time package only; protocol-specific status values are normalized by
//   the management adapter before entering this model.

package configs

import "time"

type DeliveryStatus string

const (
	DeliveryPending     DeliveryStatus = "pending"
	DeliverySent        DeliveryStatus = "sent"
	DeliveryApplying    DeliveryStatus = "applying"
	DeliveryApplied     DeliveryStatus = "applied"
	DeliveryFailed      DeliveryStatus = "failed"
	DeliveryUnsupported DeliveryStatus = "unsupported"
)

type Assignment struct {
	AgentInstanceUID  string         `json:"agent_instance_uid"`
	ConfigurationID   string         `json:"configuration_id"`
	ConfigurationHash string         `json:"configuration_hash"`
	Status            DeliveryStatus `json:"status"`
	Error             string         `json:"error,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at"`
}
