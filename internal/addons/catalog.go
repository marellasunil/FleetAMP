// Package addons defines metadata contracts for optional, external FleetAMP
// integrations. Catalog entries are descriptive only: registering an entry
// does not download, install, enable, or execute add-on code.
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
	AgentDiscovery          Capability = "agent-discovery"
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
	ID            string
	Name          string
	Description   string
	RepositoryURL string
	DocsURL       string
	Status        Status
	Bundled       bool
	Capabilities  []Capability
	Notice        string
}

type Catalog struct {
	entries map[string]Entry
}

func NewCatalog(entries ...Entry) (*Catalog, error) {
	catalog := &Catalog{entries: make(map[string]Entry, len(entries))}
	for _, entry := range entries {
		entry.ID = strings.TrimSpace(entry.ID)
		entry.Name = strings.TrimSpace(entry.Name)
		if entry.ID == "" || entry.Name == "" {
			return nil, errors.New("add-on id and name are required")
		}
		if _, exists := catalog.entries[entry.ID]; exists {
			return nil, errors.New("duplicate add-on id: " + entry.ID)
		}
		catalog.entries[entry.ID] = cloneEntry(entry)
	}
	return catalog, nil
}

func NewDefaultCatalog() *Catalog {
	catalog, err := NewCatalog(Entry{
		ID:            "alloy",
		Name:          "Grafana Alloy integration",
		Description:   "External add-on structure reserved for future Alloy interoperability documentation and capabilities.",
		RepositoryURL: "https://github.com/marellasunil/FleetAMP-Addons/tree/main/alloy",
		DocsURL:       "https://github.com/marellasunil/FleetAMP-Addons/tree/main/alloy/docs",
		Status:        Planned,
		Bundled:       false,
		Capabilities:  []Capability{ConfigurationRendering, ConfigurationValidation, AgentDiscovery, HealthReporting, RemoteConfiguration},
		Notice:        "Not installed or bundled with FleetAMP. The external repository currently contains structure only.",
	})
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
	entry.Capabilities = append([]Capability(nil), entry.Capabilities...)
	return entry
}
