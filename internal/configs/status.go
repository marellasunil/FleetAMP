// Normalized remote-configuration status report model.
//
// Purpose:
//   Carries asynchronous configuration status from a management adapter back
//   into FleetAMP assignment tracking without exposing protocol protobuf types.
//
// Flow:
//   agent status -> adapter normalization -> StatusReport -> AssignmentStore.

package configs

import "time"

// StatusReport is normalized remote-configuration status reported by an agent.
type StatusReport struct {
	AgentInstanceUID  string         `json:"agent_instance_uid"`
	ConfigurationHash string         `json:"configuration_hash"`
	Status            DeliveryStatus `json:"status"`
	Error             string         `json:"error,omitempty"`
	UpdatedAt         time.Time      `json:"updated_at"`
}
