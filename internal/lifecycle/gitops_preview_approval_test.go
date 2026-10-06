package lifecycle

import "testing"

func TestGitOpsPreviewApprovalPinsExactEvidence(t *testing.T) {
	preview := &GitOpsPreview{ID: "preview", PreviewHash: "preview-hash", PlanHash: "plan-hash", Connection: GitConnectionSnapshot{ID: "connection", Branch: "main"}, RepositoryPath: "fleetamp/groups/payments/components/request.json"}
	approval, err := NewGitOpsPreviewApproval(preview, "operator", "reviewer", "review exact content")
	if err != nil {
		t.Fatal(err)
	}
	if approval.PreviewID != preview.ID || approval.PreviewHash != preview.PreviewHash || approval.PlanHash != preview.PlanHash || approval.ConnectionID != preview.Connection.ID || approval.RepositoryPath != preview.RepositoryPath || approval.Branch != preview.Connection.Branch {
		t.Fatalf("approval did not pin evidence: %#v", approval)
	}
	if _, err := NewGitOpsPreviewApproval(preview, "operator", "operator", "self review"); err == nil {
		t.Fatal("expected four-eyes validation failure")
	}
}
