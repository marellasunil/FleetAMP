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
	OTelOperator            Type = "otel-operator"
)

type ConfigFormat string

const (
	ConfigOTelYAML ConfigFormat = "OpenTelemetry YAML"
	ConfigOTelCRD  ConfigFormat = "OpenTelemetryCollector CRD"
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
	Roles             []string
	WorkloadModes     []string
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
	r, err := NewRegistry(staticProvider{otelCollectorDescriptor()}, staticProvider{kubernetesCollectorDescriptor()}, staticProvider{otelOperatorDescriptor()})
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
	result.Roles = append([]string(nil), source.Roles...)
	result.WorkloadModes = append([]string(nil), source.WorkloadModes...)
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
		Roles:             []string{"Agent", "Gateway"},
		WorkloadModes:     []string{"System service", "Container", "DaemonSet", "Deployment", "Sidecar"},
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
		Roles:             []string{"Agent", "Gateway"},
		WorkloadModes:     []string{"DaemonSet", "Deployment", "StatefulSet", "Sidecar"},
		Capabilities: []Capability{
			{Name: "Discovery and health", Support: Planned, Detail: "Observe workload and custom-resource status."},
			{Name: "Configuration render", Support: Planned, Detail: "Render governed versions into operator resources."},
			{Name: "Drift detection", Support: Planned, Detail: "Compare desired Git or API state with cluster state."},
			{Name: "Workload deployment", Support: Planned, Detail: "Apply through an explicit GitOps or cluster integration."},
		},
	}
}

func otelOperatorDescriptor() Descriptor {
	return Descriptor{
		Type: OTelOperator, Name: "OpenTelemetry Operator",
		Description:       "Kubernetes operator for managing Collector workloads and auto-instrumentation resources.",
		ConfigFormat:      ConfigOTelCRD,
		ManagementModes:   []string{"Kubernetes API", "GitOps"},
		DeploymentMethods: []string{"Helm", "Operator manifests"},
		Roles:             []string{"Lifecycle controller"},
		WorkloadModes:     []string{"Deployment"},
		Capabilities: []Capability{
			{Name: "Installation discovery", Support: Planned, Detail: "Inventory operator versions and cluster scope."},
			{Name: "Compatibility checks", Support: Planned, Detail: "Validate Operator, Collector and CRD compatibility."},
			{Name: "Governed upgrades", Support: Planned, Detail: "Create an immutable request before changing a cluster."},
			{Name: "Workload deployment", Support: Planned, Detail: "Requires an explicit GitOps or cluster integration."},
		},
	}
}
