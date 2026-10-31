// Package addons defines provider-neutral contracts for optional FleetAMP
// telemetry destinations and Collector interoperability.
package addons

import (
	"errors"
	"sort"
	"strings"
)

type Capability string

const (
	ConfigurationRendering  Capability = "configuration-rendering"
	ConfigurationValidation Capability = "configuration-validation"
	HealthReporting         Capability = "health-reporting"
	RemoteConfiguration     Capability = "remote-configuration"
	WorkloadDeployment      Capability = "workload-deployment"
)

type Status string

const (
	Planned   Status = "planned"
	Available Status = "available"
)

type Entry struct {
	ID           string
	Name         string
	Kind         string
	Description  string
	Status       Status
	Protocols    []string
	Signals      []string
	Capabilities []Capability
	Notice       string
}

type Catalog struct {
	entries map[string]Entry
}

func NewCatalog(entries ...Entry) (*Catalog, error) {
	catalog := &Catalog{entries: make(map[string]Entry, len(entries))}
	for _, entry := range entries {
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Name = strings.TrimSpace(entry.Name)
		entry.Kind = strings.TrimSpace(entry.Kind)
		if entry.ID == "" || entry.Name == "" || entry.Kind == "" {
			return nil, errors.New("integration id, name and kind are required")
		}
		if _, exists := catalog.entries[entry.ID]; exists {
			return nil, errors.New("duplicate integration id: " + entry.ID)
		}
		catalog.entries[entry.ID] = cloneEntry(entry)
	}
	return catalog, nil
}

func NewDefaultCatalog() *Catalog {
	catalog, err := NewCatalog(
		Entry{
			ID:          "otlp-destination",
			Name:        "OTLP destination",
			Kind:        "Telemetry destination",
			Description: "Connect governed Collector pipelines to any approved backend that accepts OpenTelemetry Protocol.",
			Status:      Available,
			Protocols:   []string{"OTLP/gRPC", "OTLP/HTTP"},
			Signals:     []string{"Metrics", "Logs", "Traces", "Profiles"},
			Capabilities: []Capability{
				ConfigurationRendering, ConfigurationValidation, HealthReporting, RemoteConfiguration,
			},
			Notice: "Endpoint, transport security and authentication values are supplied through protected destination profiles and secret references.",
		},
		Entry{
			ID:          "prometheus-remote-write",
			Name:        "Prometheus Remote Write destination",
			Kind:        "Metrics destination",
			Description: "Connect governed metrics pipelines to any approved Prometheus Remote Write compatible endpoint.",
			Status:      Available,
			Protocols:   []string{"Prometheus Remote Write"},
			Signals:     []string{"Metrics"},
			Capabilities: []Capability{
				ConfigurationRendering, ConfigurationValidation, HealthReporting, RemoteConfiguration,
			},
			Notice: "FleetAMP stores organization-specific endpoints, headers and credentials outside the public capability catalog.",
		},
		Entry{
			ID:          "external-collector-adapter",
			Name:        "External Collector adapter",
			Kind:        "Collector interoperability",
			Description: "Provider-neutral contract for an externally maintained Collector or telemetry agent to declare supported management capabilities.",
			Status:      Planned,
			Protocols:   []string{"Provider-declared"},
			Signals:     []string{"Provider-declared"},
			Capabilities: []Capability{
				ConfigurationRendering, ConfigurationValidation, HealthReporting, RemoteConfiguration, WorkloadDeployment,
			},
			Notice: "Adapters remain external and unbundled. A future security boundary must verify provenance and declared capabilities before activation.",
		},
	)
	if err != nil {
		panic(err)
	}
	return catalog
}

func (c *Catalog) Get(id string) (Entry, bool) {
	entry, ok := c.entries[id]
	return cloneEntry(entry), ok
}

func (c *Catalog) List() []Entry {
	entries := make([]Entry, 0, len(c.entries))
	for _, entry := range c.entries {
		entries = append(entries, cloneEntry(entry))
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries
}

func cloneEntry(entry Entry) Entry {
	entry.Protocols = append([]string(nil), entry.Protocols...)
	entry.Signals = append([]string(nil), entry.Signals...)
	entry.Capabilities = append([]Capability(nil), entry.Capabilities...)
	return entry
}
