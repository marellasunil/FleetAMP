package agents

import "time"

// AgentType identifies the kind of telemetry agent managed by FleetAMP.
type AgentType string

const (
	AgentTypeUnknown       AgentType = "unknown"
	AgentTypeOTelCollector AgentType = "otel_collector"
	AgentTypeGrafanaAlloy  AgentType = "grafana_alloy"
)

// RuntimeType identifies where a managed agent is running. Runtime is kept
// separate from AgentType so the same agent implementation can run on a VM,
// bare metal, container platform, or Kubernetes.
type RuntimeType string

const (
	RuntimeUnknown    RuntimeType = "unknown"
	RuntimeVM         RuntimeType = "vm"
	RuntimeBareMetal  RuntimeType = "bare_metal"
	RuntimeContainer  RuntimeType = "container"
	RuntimeKubernetes RuntimeType = "kubernetes"
)

// DeploymentContext describes the execution environment without coupling the
// core domain model to a specific cloud or orchestrator.
type DeploymentContext struct {
	Runtime RuntimeType `json:"runtime"`

	// Provider is optional and may contain values such as aws, azure, gcp,
	// on_prem, or another deployment provider.
	Provider string `json:"provider,omitempty"`

	// Cluster is useful for Kubernetes or other clustered runtimes.
	Cluster string `json:"cluster,omitempty"`

	// Namespace is primarily useful for Kubernetes-style runtimes.
	Namespace string `json:"namespace,omitempty"`

	// Node identifies the runtime node when available.
	Node string `json:"node,omitempty"`
}

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

	// Deployment describes where the agent runs. FleetAMP manages the
	// telemetry-agent control plane; the underlying runtime remains responsible
	// for workload scheduling and replica lifecycle.
	Deployment DeploymentContext `json:"deployment"`

	// Attributes are metadata reported by the managed agent/protocol.
	// Examples: service.name, host.name, os.type, cloud.region,
	// k8s.cluster.name, or k8s.namespace.name.
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
