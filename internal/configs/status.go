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
