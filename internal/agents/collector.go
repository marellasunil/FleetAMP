package agents

import "time"

// AgentType identifies the kind of telemetry agent managed by FleetAMP.
type AgentType string

const (
	AgentTypeUnknown       AgentType = "unknown"
	AgentTypeOTelCollector AgentType = "otel_collector"
	AgentTypeGrafanaAlloy  AgentType = "grafana_alloy"
)

// ManagedAgent is FleetAMP's protocol-independent representation of a
// telemetry agent. Protocol-specific types (for example OpAMP protobufs)
// must be translated into this model at the integration boundary.
type ManagedAgent struct {
	InstanceUID string    `json:"instance_uid"`
	Type        AgentType `json:"type"`

	Name     string `json:"name,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Version  string `json:"version,omitempty"`

	Connected bool      `json:"connected"`
	Healthy   bool      `json:"healthy"`
	LastSeen  time.Time `json:"last_seen"`

	// Attributes are metadata reported by the managed agent/protocol.
	// Examples: service.name, host.name, os.type, cloud.region.
	Attributes map[string]string `json:"attributes,omitempty"`

	// Labels are FleetAMP-owned management metadata used for grouping,
	// targeting and policy. Examples: team, environment, role, region.
	Labels map[string]string `json:"labels,omitempty"`

	// Capabilities contains normalized management capabilities advertised
	// by the agent, such as reports_health or accepts_remote_config.
	Capabilities []string `json:"capabilities,omitempty"`
}

// Touch updates the agent's last-seen timestamp.
func (a *ManagedAgent) Touch() {
	a.LastSeen = time.Now().UTC()
}

// Collector is kept as a compatibility alias while FleetAMP evolves from an
// OTel-Collector-only model to the generic ManagedAgent domain model.
type Collector = ManagedAgent
