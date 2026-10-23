package distributions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// PreviewHandoff is the immutable evidence joining a released distribution
// preview to the exact configuration version and approval request it created.
type PreviewHandoff struct {
	ID                  string    `json:"id"`
	PreviewID           string    `json:"preview_id"`
	PreviewContentHash  string    `json:"preview_content_hash"`
	ReleaseID           string    `json:"release_id"`
	ReleaseEvidenceHash string    `json:"release_evidence_hash"`
	ArtifactSHA256      string    `json:"artifact_sha256"`
	GroupID             string    `json:"group_id"`
	ConfigurationID     string    `json:"configuration_id"`
	ConfigurationHash   string    `json:"configuration_hash"`
	ApprovalRequestID   string    `json:"approval_request_id"`
	SubmittedBy         string    `json:"submitted_by"`
	EvidenceHash        string    `json:"evidence_hash"`
	CreatedAt           time.Time `json:"created_at"`
}

func NewPreviewHandoff(preview *DeploymentPreview, configurationID, configurationHash, approvalRequestID, actor string, now time.Time) (*PreviewHandoff, error) {
	if preview == nil {
		return nil, errors.New("distribution preview is required")
	}
	h := &PreviewHandoff{PreviewID: preview.ID, PreviewContentHash: preview.ContentHash, ReleaseID: preview.ReleaseID, ReleaseEvidenceHash: preview.ReleaseEvidenceHash, ArtifactSHA256: preview.ArtifactSHA256, GroupID: preview.GroupID, ConfigurationID: strings.TrimSpace(configurationID), ConfigurationHash: strings.TrimSpace(configurationHash), ApprovalRequestID: strings.TrimSpace(approvalRequestID), SubmittedBy: strings.TrimSpace(actor), CreatedAt: now.UTC()}
	if h.PreviewID == "" || h.PreviewContentHash == "" || h.ReleaseID == "" || h.ReleaseEvidenceHash == "" || h.ArtifactSHA256 == "" || h.GroupID == "" || h.ConfigurationID == "" || h.ConfigurationHash == "" || h.ApprovalRequestID == "" || h.SubmittedBy == "" {
		return nil, errors.New("complete preview, release, configuration, approval and actor evidence is required")
	}
	canonical, _ := json.Marshal(struct{ PreviewID, PreviewContentHash, ReleaseID, ReleaseEvidenceHash, ArtifactSHA256, GroupID, ConfigurationID, ConfigurationHash, ApprovalRequestID, SubmittedBy string }{h.PreviewID, h.PreviewContentHash, h.ReleaseID, h.ReleaseEvidenceHash, h.ArtifactSHA256, h.GroupID, h.ConfigurationID, h.ConfigurationHash, h.ApprovalRequestID, h.SubmittedBy})
	sum := sha256.Sum256(canonical)
	h.EvidenceHash = hex.EncodeToString(sum[:])
	h.ID = "dist-handoff-" + h.EvidenceHash[:16]
	return h, nil
}
