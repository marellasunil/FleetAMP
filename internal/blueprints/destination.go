// Package blueprints contains the governed inputs used by FleetAMP's guided
// OpenTelemetry configuration builder.
package blueprints

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// DestinationProfile is an administrator-approved exporter destination. The
// exporter configuration is deliberately opaque to ordinary users so endpoint,
// authentication and TLS policy remain centrally controlled.
type DestinationProfile struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Environment    string    `json:"environment"`
	ExporterID     string    `json:"exporter_id"`
	ExporterConfig string    `json:"exporter_config"`
	Owner          string    `json:"owner"`
	Visibility     string    `json:"visibility"`
	GroupIDs       []string  `json:"group_ids"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func NewDestinationProfile(name, environment, exporterID, exporterConfig string) *DestinationProfile {
	now := time.Now().UTC()
	name, environment = strings.TrimSpace(name), strings.TrimSpace(environment)
	sum := sha256.Sum256([]byte(strings.ToLower(name + "\x00" + environment)))
	return &DestinationProfile{ID: hex.EncodeToString(sum[:12]), Name: name, Environment: environment,
		ExporterID: strings.TrimSpace(exporterID), ExporterConfig: strings.TrimSpace(exporterConfig),
		Owner: "Platform Team", Visibility: "organization", GroupIDs: []string{},
		Enabled: true, CreatedAt: now, UpdatedAt: now}
}

// AllowsGroup enforces the catalog visibility boundary independently of the UI.
func (p *DestinationProfile) AllowsGroup(groupID string) bool {
	if p == nil || !p.Enabled {
		return false
	}
	if p.Visibility == "organization" {
		return true
	}
	for _, allowed := range p.GroupIDs {
		if allowed == groupID {
			return true
		}
	}
	return false
}
