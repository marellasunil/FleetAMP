// Package distributions models administrator-published OpenTelemetry Collector
// build specifications. Records are immutable: a correction is a new version.
package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

const OfficialContribRepository = "https://github.com/open-telemetry/opentelemetry-collector-releases"

type ComponentSet struct {
	Receivers  []string `json:"receivers"`
	Processors []string `json:"processors"`
	Exporters  []string `json:"exporters"`
	Extensions []string `json:"extensions"`
	Connectors []string `json:"connectors"`
}

type Distribution struct {
	ID               string       `json:"id"`
	Name             string       `json:"name"`
	CollectorVersion string       `json:"collector_version"`
	SourceRepository string       `json:"source_repository"`
	SourceRef        string       `json:"source_ref"`
	ArtifactURI      string       `json:"artifact_uri"`
	ArtifactSHA256   string       `json:"artifact_sha256"`
	Components       ComponentSet `json:"components"`
	BaselineConfig   string       `json:"baseline_config"`
	BaselineHash     string       `json:"baseline_hash"`
	LockedSections   []string     `json:"locked_sections"`
	EditableSections []string     `json:"editable_sections"`
	SpecHash         string       `json:"spec_hash"`
	PublishedBy      string       `json:"published_by"`
	PublishedAt      time.Time    `json:"published_at"`
}

func New(name, version, sourceRef, artifactURI, artifactSHA string, components ComponentSet, baseline string, locked, editable []string, actor string, now time.Time) (*Distribution, error) {
	baseline = strings.TrimSpace(baseline)
	if baseline != "" { baseline += "\n" }
	baselineSum := sha256.Sum256([]byte(baseline))
	d := &Distribution{Name: strings.TrimSpace(name), CollectorVersion: strings.TrimSpace(version), SourceRepository: OfficialContribRepository, SourceRef: strings.TrimSpace(sourceRef), ArtifactURI: strings.TrimSpace(artifactURI), ArtifactSHA256: strings.ToLower(strings.TrimSpace(artifactSHA)), Components: normalizeComponents(components), BaselineConfig: baseline, BaselineHash: hex.EncodeToString(baselineSum[:]), LockedSections: normalize(locked), EditableSections: normalize(editable), PublishedBy: strings.TrimSpace(actor), PublishedAt: now.UTC()}
	if err := d.Validate(); err != nil { return nil, err }
	canonical, _ := json.Marshal(struct { Name, Version, Repository, Ref, URI, SHA, BaselineHash string; Components ComponentSet; Locked, Editable []string }{d.Name, d.CollectorVersion, d.SourceRepository, d.SourceRef, d.ArtifactURI, d.ArtifactSHA256, d.BaselineHash, d.Components, d.LockedSections, d.EditableSections})
	sum := sha256.Sum256(canonical)
	d.SpecHash = hex.EncodeToString(sum[:])
	d.ID = "dist-" + d.SpecHash[:16]
	return d, nil
}

func (d *Distribution) Validate() error {
	if d.Name == "" || d.CollectorVersion == "" || d.SourceRef == "" { return errors.New("name, Collector version and pinned source reference are required") }
	if d.SourceRef == "latest" || strings.HasSuffix(d.SourceRef, "/main") || d.SourceRef == "main" { return errors.New("source reference must be an immutable tag or commit, not latest/main") }
	if d.ArtifactSHA256 != "" && (len(d.ArtifactSHA256) != 64 || !isHex(d.ArtifactSHA256)) { return errors.New("artifact SHA-256 must contain exactly 64 hexadecimal characters") }
	if componentCount(d.Components) == 0 { return errors.New("at least one Collector component is required") }
	if strings.TrimSpace(d.BaselineConfig) == "" { return errors.New("organization baseline Collector configuration is required") }
	editable := map[string]bool{}; for _, section := range d.EditableSections { editable[section] = true }
	for _, section := range d.LockedSections { if editable[section] { return errors.New("a configuration section cannot be both locked and editable") } }
	return nil
}

func normalizeComponents(c ComponentSet) ComponentSet { c.Receivers=normalize(c.Receivers); c.Processors=normalize(c.Processors); c.Exporters=normalize(c.Exporters); c.Extensions=normalize(c.Extensions); c.Connectors=normalize(c.Connectors); return c }
func normalize(values []string) []string { seen:=map[string]bool{}; out:=[]string{}; for _, value:=range values { value=strings.TrimSpace(value); if value!=""&&!seen[value] { seen[value]=true; out=append(out,value) } }; sort.Strings(out); return out }
func componentCount(c ComponentSet) int { return len(c.Receivers)+len(c.Processors)+len(c.Exporters)+len(c.Extensions)+len(c.Connectors) }
func isHex(value string) bool { _, err := hex.DecodeString(value); return err == nil }
