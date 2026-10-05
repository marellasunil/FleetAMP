// Package integrations defines provider-neutral contracts for external systems.
// This foundation is descriptive only and does not store credentials, call a
// provider API, create repositories, or create pipelines.
package integrations

import (
	"errors"
	"sort"
	"strings"
)

type ProviderID string

const (
	GitHub           ProviderID = "github"
	GitLab           ProviderID = "gitlab"
	AzureDevOps      ProviderID = "azure-devops"
	ModeGitManaged              = "git-managed"
	ModePullRequest             = "fleetamp-pull-request"
	ModeDirectCommit            = "fleetamp-direct-commit"
	ModePrimarySync             = "fleetamp-primary-sync"
	ModeObserveOnly             = "observe-only"
)

type Capability string

const (
	ReadRepository      Capability = "read-repository"
	ReceiveWebhook      Capability = "receive-webhook"
	WriteConfiguration  Capability = "write-configuration"
	CreateChangeRequest Capability = "create-change-request"
	ReportStatus        Capability = "report-status"
)

type Provider struct {
	ID             ProviderID
	Name           string
	Description    string
	ChangeRequest  string
	AuthOptions    []string
	Capabilities   []Capability
	SupportedModes []string
	Status         string
}

type Catalog struct {
	providers map[ProviderID]Provider
}

func NewCatalog(providers ...Provider) (*Catalog, error) {
	catalog := &Catalog{providers: make(map[ProviderID]Provider, len(providers))}
	for _, provider := range providers {
		provider.Name = strings.TrimSpace(provider.Name)
		if provider.ID == "" || provider.Name == "" {
			return nil, errors.New("integration provider id and name are required")
		}
		if _, exists := catalog.providers[provider.ID]; exists {
			return nil, errors.New("duplicate integration provider: " + string(provider.ID))
		}
		catalog.providers[provider.ID] = cloneProvider(provider)
	}
	return catalog, nil
}

func NewDefaultGitCatalog() *Catalog {
	sharedCapabilities := []Capability{ReadRepository, ReceiveWebhook, WriteConfiguration, CreateChangeRequest, ReportStatus}
	sharedModes := []string{ModeGitManaged, ModePullRequest, ModeDirectCommit, ModePrimarySync, ModeObserveOnly}
	catalog, err := NewCatalog(
		Provider{ID: GitHub, Name: "GitHub", Description: "Import approved repository changes and write FleetAMP-managed changes through pull requests or commits.", ChangeRequest: "Pull request", AuthOptions: []string{"GitHub App", "Fine-grained token"}, Capabilities: sharedCapabilities, SupportedModes: sharedModes, Status: "Not connected"},
		Provider{ID: GitLab, Name: "GitLab", Description: "Connect GitLab.com or self-managed projects for governed import, merge requests, status, and synchronization.", ChangeRequest: "Merge request", AuthOptions: []string{"OAuth application", "Project access token"}, Capabilities: sharedCapabilities, SupportedModes: sharedModes, Status: "Not connected"},
		Provider{ID: AzureDevOps, Name: "Azure DevOps", Description: "Connect an existing Azure Repos project for governed import, pull requests, status, and synchronization.", ChangeRequest: "Pull request", AuthOptions: []string{"Service principal", "Personal access token"}, Capabilities: sharedCapabilities, SupportedModes: sharedModes, Status: "Not connected"},
	)
	if err != nil {
		panic(err)
	}
	return catalog
}

func (c *Catalog) Get(id ProviderID) (Provider, bool) {
	provider, ok := c.providers[id]
	return cloneProvider(provider), ok
}

func (c *Catalog) List() []Provider {
	providers := make([]Provider, 0, len(c.providers))
	for _, provider := range c.providers {
		providers = append(providers, cloneProvider(provider))
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name < providers[j].Name })
	return providers
}

func cloneProvider(provider Provider) Provider {
	provider.AuthOptions = append([]string(nil), provider.AuthOptions...)
	provider.Capabilities = append([]Capability(nil), provider.Capabilities...)
	provider.SupportedModes = append([]string(nil), provider.SupportedModes...)
	return provider
}
