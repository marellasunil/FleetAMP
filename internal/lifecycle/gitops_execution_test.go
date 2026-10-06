package lifecycle

import "testing"

func TestGitOpsExecutionRequiresExactApprovedPreview(t *testing.T) {
	preview := &GitOpsPreview{ID: "preview", PreviewHash: "preview-hash", PlanHash: "plan-hash", RepositoryPath: "fleetamp/groups/payments/change.json", Connection: GitConnectionSnapshot{ID: "connection", Provider: "github", Mode: "fleetamp-pull-request", Branch: "main"}}
	approval := &GitOpsPreviewApproval{ID: "approval", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, PlanHash: preview.PlanHash, ConnectionID: preview.Connection.ID, RepositoryPath: preview.RepositoryPath, Branch: preview.Connection.Branch, Status: ApprovalApproved}
	request, event, err := NewGitOpsExecutionRequest(approval, preview, "operator")
	if err != nil {
		t.Fatal(err)
	}
	if request.ApprovalID != approval.ID || request.PreviewHash != preview.PreviewHash || event.Status != GitOpsExecutionQueued {
		t.Fatalf("execution evidence mismatch: %#v %#v", request, event)
	}
	approval.PreviewHash = "different"
	if _, _, err := NewGitOpsExecutionRequest(approval, preview, "operator"); err == nil {
		t.Fatal("expected mismatched preview evidence to be rejected")
	}
}

func TestGitOpsExecutionRejectsUnapprovedPreview(t *testing.T) {
	preview := &GitOpsPreview{ID: "preview", PreviewHash: "preview-hash", PlanHash: "plan-hash", RepositoryPath: "path", Connection: GitConnectionSnapshot{ID: "connection", Provider: "github", Mode: "fleetamp-pull-request", Branch: "main"}}
	approval := &GitOpsPreviewApproval{ID: "approval", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, PlanHash: preview.PlanHash, ConnectionID: preview.Connection.ID, RepositoryPath: preview.RepositoryPath, Branch: preview.Connection.Branch, Status: ApprovalPending}
	if _, _, err := NewGitOpsExecutionRequest(approval, preview, "operator"); err == nil {
		t.Fatal("expected pending approval to be rejected")
	}
}
