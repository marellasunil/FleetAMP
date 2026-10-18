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
	GroupID              string            `json:"group_id"`
	GroupName            string            `json:"group_name"`
	Sections             map[string]string `json:"sections"`
	ComposedConfig       string            `json:"composed_config"`
	ContentHash          string            `json:"content_hash"`
	ValidationEvidence   string            `json:"validation_evidence"`
	PreparedBy           string            `json:"prepared_by"`
	CreatedAt            time.Time         `json:"created_at"`
}

func NewDeploymentPreview(distribution *Distribution, groupID, groupName string, sections map[string]string, composed, validationEvidence, actor string, now time.Time) (*DeploymentPreview, error) {
	if distribution == nil { return nil, errors.New("distribution is required") }
	preview := &DeploymentPreview{DistributionID: distribution.ID, DistributionSpecHash: distribution.SpecHash, GroupID: strings.TrimSpace(groupID), GroupName: strings.TrimSpace(groupName), Sections: sections, ComposedConfig: strings.TrimSpace(composed)+"\n", ValidationEvidence: strings.TrimSpace(validationEvidence), PreparedBy: strings.TrimSpace(actor), CreatedAt: now.UTC()}
	if preview.GroupID == "" || preview.GroupName == "" || strings.TrimSpace(preview.ComposedConfig) == "" || preview.PreparedBy == "" { return nil, errors.New("group, composed configuration and preparer are required") }
	contentSum := sha256.Sum256([]byte(preview.ComposedConfig)); preview.ContentHash = hex.EncodeToString(contentSum[:])
	canonical, _ := json.Marshal(struct{ DistributionSpecHash, GroupID, ContentHash, ValidationEvidence string }{preview.DistributionSpecHash, preview.GroupID, preview.ContentHash, preview.ValidationEvidence})
	sum := sha256.Sum256(canonical); preview.ID = "dist-preview-"+hex.EncodeToString(sum[:8])
	return preview,nil
}
