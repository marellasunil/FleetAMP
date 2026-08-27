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
