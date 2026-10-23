package distributions

import (
	"strings"
	"testing"
	"time"
)

func TestPreviewHandoffBindsExactEvidence(t *testing.T) {
	preview := &DeploymentPreview{ID: "dist-preview-1", ContentHash: strings.Repeat("a", 64), ReleaseID: "release-1", ReleaseEvidenceHash: strings.Repeat("b", 64), ArtifactSHA256: strings.Repeat("c", 64), GroupID: "group-1"}
	handoff, err := NewPreviewHandoff(preview, "config-1", strings.Repeat("d", 64), "approval-1", "operator", time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if handoff.PreviewID != preview.ID || handoff.ConfigurationID != "config-1" || handoff.ApprovalRequestID != "approval-1" {
		t.Fatalf("unexpected handoff: %#v", handoff)
	}
	if len(handoff.EvidenceHash) != 64 || handoff.ID != "dist-handoff-"+handoff.EvidenceHash[:16] {
		t.Fatalf("invalid immutable identity: %#v", handoff)
	}
	changed := *preview
	changed.ArtifactSHA256 = strings.Repeat("e", 64)
	second, err := NewPreviewHandoff(&changed, "config-1", strings.Repeat("d", 64), "approval-1", "operator", time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if second.EvidenceHash == handoff.EvidenceHash {
		t.Fatal("artifact substitution must change handoff evidence")
	}
}

func TestPreviewHandoffRejectsIncompleteEvidence(t *testing.T) {
	_, err := NewPreviewHandoff(&DeploymentPreview{ID: "preview"}, "config", "hash", "approval", "operator", time.Now())
	if err == nil {
		t.Fatal("expected incomplete preview evidence to be rejected")
	}
}
