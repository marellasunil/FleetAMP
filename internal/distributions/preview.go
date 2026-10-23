package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type DeploymentPreview struct {
	ID                   string            `json:"id"`
	DistributionID       string            `json:"distribution_id"`
	DistributionSpecHash string            `json:"distribution_spec_hash"`
	ReleaseID            string            `json:"release_id"`
	ReleaseEvidenceHash  string            `json:"release_evidence_hash"`
	ArtifactURI          string            `json:"artifact_uri"`
	ArtifactSHA256       string            `json:"artifact_sha256"`
	GroupID              string            `json:"group_id"`
	GroupName            string            `json:"group_name"`
	Sections             map[string]string `json:"sections"`
	ComposedConfig       string            `json:"composed_config"`
	ContentHash          string            `json:"content_hash"`
	ValidationEvidence   string            `json:"validation_evidence"`
	PreparedBy           string            `json:"prepared_by"`
	CreatedAt            time.Time         `json:"created_at"`
}

func NewDeploymentPreview(distribution *Distribution, release *DistributionRelease, groupID, groupName string, sections map[string]string, composed, validationEvidence, actor string, now time.Time) (*DeploymentPreview, error) {
	if distribution == nil {
		return nil, errors.New("distribution is required")
	}
	releaseID, releaseEvidence, artifactURI, artifactSHA := "", "", "", ""
	if release != nil {
		if release.DistributionID != distribution.ID || release.DistributionSpecHash != distribution.SpecHash || release.ArtifactURI == "" || release.ArtifactSHA256 == "" {
			return nil, errors.New("release does not match the exact distribution specification")
		}
		releaseID, releaseEvidence, artifactURI, artifactSHA = release.ID, release.EvidenceHash, release.ArtifactURI, release.ArtifactSHA256
	} else if distribution.ArtifactURI != "" && distribution.ArtifactSHA256 != "" {
		releaseID, releaseEvidence, artifactURI, artifactSHA = "legacy-"+distribution.ID, distribution.SpecHash, distribution.ArtifactURI, distribution.ArtifactSHA256
	} else {
		return nil, errors.New("a finalized distribution release is required")
	}
	preview := &DeploymentPreview{DistributionID: distribution.ID, DistributionSpecHash: distribution.SpecHash, ReleaseID: releaseID, ReleaseEvidenceHash: releaseEvidence, ArtifactURI: artifactURI, ArtifactSHA256: artifactSHA, GroupID: strings.TrimSpace(groupID), GroupName: strings.TrimSpace(groupName), Sections: sections, ComposedConfig: strings.TrimSpace(composed) + "\n", ValidationEvidence: strings.TrimSpace(validationEvidence), PreparedBy: strings.TrimSpace(actor), CreatedAt: now.UTC()}
	if preview.GroupID == "" || preview.GroupName == "" || strings.TrimSpace(preview.ComposedConfig) == "" || preview.PreparedBy == "" {
		return nil, errors.New("group, composed configuration and preparer are required")
	}
	contentSum := sha256.Sum256([]byte(preview.ComposedConfig))
	preview.ContentHash = hex.EncodeToString(contentSum[:])
	canonical, _ := json.Marshal(struct{ DistributionSpecHash, ReleaseID, ReleaseEvidenceHash, ArtifactURI, ArtifactSHA256, GroupID, ContentHash, ValidationEvidence string }{preview.DistributionSpecHash, preview.ReleaseID, preview.ReleaseEvidenceHash, preview.ArtifactURI, preview.ArtifactSHA256, preview.GroupID, preview.ContentHash, preview.ValidationEvidence})
	sum := sha256.Sum256(canonical)
	preview.ID = "dist-preview-" + hex.EncodeToString(sum[:8])
	return preview, nil
}
