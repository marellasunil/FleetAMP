// Package runtimes describes telemetry runtimes FleetAMP can govern.
//
// The catalog is intentionally integration-neutral. Runtime-specific clients,
// credentials, renderers, and deployment operations belong behind providers
// added in later milestones.
package runtimes

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

type Type string

const (
	OTelCollector           Type = "otel-collector"
	OTelCollectorKubernetes Type = "otel-collector-kubernetes"
	GrafanaAlloy            Type = "grafana-alloy"
)

type ConfigFormat string

const (
	ConfigOTelYAML ConfigFormat = "OpenTelemetry YAML"
	ConfigOTelCRD  ConfigFormat = "OpenTelemetryCollector CRD"
	ConfigAlloy    ConfigFormat = "Alloy configuration"
)

type Support string

const (
	Supported Support = "supported"
	Planned   Support = "planned"
)

type Capability struct {
	Name    string
	Support Support
	Detail  string
}

type Descriptor struct {
	Type              Type
	Name              string
	Description       string
	ConfigFormat      ConfigFormat
	ManagementModes   []string
	DeploymentMethods []string
	Capabilities      []Capability
}

type Provider interface {
	Descriptor() Descriptor
}

var ErrDuplicateProvider = errors.New("runtime provider already registered")

type Registry struct {
	mu        sync.RWMutex
	providers map[Type]Provider
}

func NewRegistry(providers ...Provider) (*Registry, error) {
	r := &Registry{providers: make(map[Type]Provider, len(providers))}
	for _, provider := range providers {
		if err := r.Register(provider); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func NewDefaultRegistry() *Registry {
	r, err := NewRegistry(staticProvider{otelCollectorDescriptor()}, staticProvider{kubernetesCollectorDescriptor()}, staticProvider{alloyDescriptor()})
	if err != nil {
		panic(err)
	}
	return r
}

func (r *Registry) Register(provider Provider) error {
	if provider == nil {
		return errors.New("runtime provider is required")
	}
	descriptor := provider.Descriptor()
	if descriptor.Type == "" || strings.TrimSpace(descriptor.Name) == "" {
		return errors.New("runtime provider type and name are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[descriptor.Type]; exists {
		return ErrDuplicateProvider
	}
	r.providers[descriptor.Type] = provider
	return nil
}

func (r *Registry) Get(runtimeType Type) (Descriptor, bool) {
	r.mu.RLock()
	provider, ok := r.providers[runtimeType]
	r.mu.RUnlock()
	if !ok {
		return Descriptor{}, false
	}
	return cloneDescriptor(provider.Descriptor()), true
}

func (r *Registry) List() []Descriptor {
	r.mu.RLock()
	descriptors := make([]Descriptor, 0, len(r.providers))
	for _, provider := range r.providers {
		descriptors = append(descriptors, cloneDescriptor(provider.Descriptor()))
	}
	r.mu.RUnlock()
	sort.Slice(descriptors, func(i, j int) bool { return descriptors[i].Name < descriptors[j].Name })
	return descriptors
}

type staticProvider struct{ descriptor Descriptor }

func (p staticProvider) Descriptor() Descriptor { return cloneDescriptor(p.descriptor) }

func cloneDescriptor(source Descriptor) Descriptor {
	result := source
	result.ManagementModes = append([]string(nil), source.ManagementModes...)
	result.DeploymentMethods = append([]string(nil), source.DeploymentMethods...)
	result.Capabilities = append([]Capability(nil), source.Capabilities...)
	return result
}

func otelCollectorDescriptor() Descriptor {
	return Descriptor{
		Type: OTelCollector, Name: "OpenTelemetry Collector",
		Description:       "Collector processes managed through FleetAMP's existing OpAMP control plane.",
		ConfigFormat:      ConfigOTelYAML,
		ManagementModes:   []string{"OpAMP"},
		DeploymentMethods: []string{"Binary", "Container", "Kubernetes workload"},
		Capabilities: []Capability{
			{Name: "Discovery and health", Support: Supported, Detail: "Reported through OpAMP."},
			{Name: "Remote configuration", Support: Supported, Detail: "Governed version delivery through OpAMP."},
			{Name: "Drift and rollback", Support: Supported, Detail: "Existing FleetAMP lifecycle workflow."},
			{Name: "Workload deployment", Support: Planned, Detail: "Runtime installation remains external today."},
		},
	}
}

func kubernetesCollectorDescriptor() Descriptor {
	return Descriptor{
		Type: OTelCollectorKubernetes, Name: "Kubernetes OTel Collector",
		Description:       "Operator-managed collectors represented as Kubernetes custom resources.",
		ConfigFormat:      ConfigOTelCRD,
		ManagementModes:   []string{"OpenTelemetry Operator", "GitOps"},
		DeploymentMethods: []string{"Helm", "Custom resource", "Kustomize"},
		Capabilities: []Capability{
			{Name: "Discovery and health", Support: Planned, Detail: "Observe workload and custom-resource status."},
			{Name: "Configuration render", Support: Planned, Detail: "Render governed versions into operator resources."},
			{Name: "Drift detection", Support: Planned, Detail: "Compare desired Git or API state with cluster state."},
			{Name: "Workload deployment", Support: Planned, Detail: "Apply through an explicit GitOps or cluster integration."},
		},
	}
}

func alloyDescriptor() Descriptor {
	return Descriptor{
		Type: GrafanaAlloy, Name: "Grafana Alloy",
		Description:       "Alloy agents governed through native remote configuration or its OTel Engine OpAMP support.",
		ConfigFormat:      ConfigAlloy,
		ManagementModes:   []string{"Alloy remote configuration", "Git / HTTP import", "OTel Engine OpAMP"},
		DeploymentMethods: []string{"Binary", "Container", "Helm"},
		Capabilities: []Capability{
			{Name: "Discovery and health", Support: Planned, Detail: "Normalize Alloy identity and health into the fleet model."},
			{Name: "Configuration validation", Support: Planned, Detail: "Validate Alloy syntax before version creation."},
			{Name: "Remote configuration", Support: Planned, Detail: "Provider will select the configured Alloy management mode."},
			{Name: "Workload deployment", Support: Planned, Detail: "Install through GitOps or an explicit deployment integration."},
		},
	}
}
