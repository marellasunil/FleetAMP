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

const BuildStatusPlanned = "planned"

type BuildManifest struct {
	DistributionID       string       `json:"distribution_id"`
	DistributionSpecHash string       `json:"distribution_spec_hash"`
	CollectorVersion     string       `json:"collector_version"`
	SourceRepository     string       `json:"source_repository"`
	SourceRef            string       `json:"source_ref"`
	Components           ComponentSet `json:"components"`
	BaselineHash         string       `json:"baseline_hash"`
	Outputs              []string     `json:"outputs"`
	DestinationRef       string       `json:"destination_ref"`
	CredentialRef        string       `json:"credential_ref"`
}

type BuildPlan struct {
	ID             string        `json:"id"`
	DistributionID string        `json:"distribution_id"`
	Manifest       BuildManifest `json:"manifest"`
	ManifestJSON   string        `json:"manifest_json"`
	PlanHash       string        `json:"plan_hash"`
	Status         string        `json:"status"`
	RequestedBy    string        `json:"requested_by"`
	RequestedAt    time.Time     `json:"requested_at"`
}

func NewBuildPlan(distribution *Distribution, outputs []string, destinationRef, credentialRef, actor string, now time.Time) (*BuildPlan, error) {
	if distribution == nil {
		return nil, errors.New("distribution is required")
	}
	outputs = normalize(outputs)
	for _, output := range outputs {
		if output != "oci-image" && output != "binary-archive" {
			return nil, errors.New("unsupported build output: " + output)
		}
	}
	if len(outputs) == 0 {
		return nil, errors.New("at least one build output is required")
	}
	destinationRef = strings.TrimSpace(destinationRef)
	credentialRef = strings.TrimSpace(credentialRef)
	actor = strings.TrimSpace(actor)
	if destinationRef == "" || (!strings.HasPrefix(destinationRef, "oci://") && !strings.HasPrefix(destinationRef, "artifact://")) {
		return nil, errors.New("destination must use an oci:// or artifact:// reference")
	}
	if credentialRef == "" || !strings.HasPrefix(credentialRef, "secret://") {
		return nil, errors.New("credential must use a secret:// reference")
	}
	manifest := BuildManifest{DistributionID: distribution.ID, DistributionSpecHash: distribution.SpecHash, CollectorVersion: distribution.CollectorVersion, SourceRepository: distribution.SourceRepository, SourceRef: distribution.SourceRef, Components: distribution.Components, BaselineHash: distribution.BaselineHash, Outputs: outputs, DestinationRef: destinationRef, CredentialRef: credentialRef}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(manifestJSON)
	hash := hex.EncodeToString(sum[:])
	return &BuildPlan{ID: "build-" + hash[:16], DistributionID: distribution.ID, Manifest: manifest, ManifestJSON: string(manifestJSON), PlanHash: hash, Status: BuildStatusPlanned, RequestedBy: actor, RequestedAt: now.UTC()}, nil
}

func NormalizeBuildOutputs(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
