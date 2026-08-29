// Vendor-neutral external configuration provider contracts.
//
// Purpose:
//
//	Defines references and fetch behavior for future GitHub, Azure DevOps,
//	GitLab, filesystem, or other desired-configuration sources.
package providers

import "context"

// ConfigRef identifies a configuration artifact without coupling FleetAMP to
// a specific source-control or CI/CD vendor.
type ConfigRef struct {
	Provider   string            `json:"provider"`
	Repository string            `json:"repository,omitempty"`
	Revision   string            `json:"revision,omitempty"`
	Path       string            `json:"path,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ConfigProvider retrieves a Collector/agent configuration from an external
// source such as Azure DevOps, GitHub, GitLab or a local filesystem.
type ConfigProvider interface {
	Name() string
	Fetch(ctx context.Context, ref ConfigRef) ([]byte, error)
}
