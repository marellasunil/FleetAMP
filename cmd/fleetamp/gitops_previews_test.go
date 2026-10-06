package main

import (
	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGitOpsPreviewPageRendersWithoutGitWrite(t *testing.T) {
	requests := memory.NewComponentLifecycleRequestStore()
	plans := memory.NewComponentLifecycleExecutionStore()
	previews := memory.NewComponentGitOpsPreviewStore()
	previewApprovals := memory.NewGitOpsPreviewApprovalStore()
	connections := memory.NewIntegrationConnectionStore()
	request, _ := lifecycle.NewRequest(lifecycle.Spec{Operation: lifecycle.Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security"}, "operator")
	_ = requests.Create(t.Context(), request)
	plan := &lifecycle.ExecutionPlan{ID: "plan", RequestID: request.ID, RequestSpecHash: request.SpecHash, ExecutorKind: lifecycle.ExecutorGitOps, PlanHash: "plan-hash"}
	_ = plans.Create(t.Context(), plan)
	connection, _ := integrations.NewConnection(integrations.Connection{Name: "production", Provider: integrations.GitHub, Organization: "acme", Repository: "telemetry", Branch: "main", AllowedRoot: "fleetamp/groups", Mode: integrations.ModePullRequest, CredentialRef: "secret://github/prod", GroupIDs: []string{"payments"}, Enabled: true}, "admin")
	_ = connections.Create(t.Context(), connection)
	mux := http.NewServeMux()
	registerGitOpsPreviewRoutes(mux, plans, requests, previews, previewApprovals, connections, nil, nil)
	form := url.Values{"plan_id": {"plan"}, "connection_id": {connection.ID}}
	response := httptest.NewRecorder()
	post := httptest.NewRequest(http.MethodPost, "/component-gitops-previews", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, post)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/component-gitops-previews", nil))
	for _, want := range []string{"GitOps proposal previews", "Preview SHA-256", "production", "fleetamp/groups/payments/components/", "No repository, branch, commit, pull request, cluster or component was changed."} {
		if !strings.Contains(get.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
	previewRows, _ := previews.List(t.Context(), 10)
	submitForm := url.Values{"action": {"submit_preview_approval"}, "preview_id": {previewRows[0].ID}, "assigned_reviewer": {"reviewer"}, "submission_comment": {"review exact repository evidence"}}
	submitResponse := httptest.NewRecorder()
	submit := httptest.NewRequest(http.MethodPost, "/component-gitops-previews", strings.NewReader(submitForm.Encode()))
	submit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(submitResponse, submit)
	if submitResponse.Code != http.StatusSeeOther {
		t.Fatalf("submit status=%d body=%s", submitResponse.Code, submitResponse.Body.String())
	}
	approvalRows, _ := previewApprovals.List(t.Context(), 10)
	if len(approvalRows) != 1 || approvalRows[0].PreviewHash != previewRows[0].PreviewHash || approvalRows[0].ConnectionID != connection.ID {
		t.Fatalf("approval does not pin preview evidence: %#v", approvalRows)
	}
	reviewForm := url.Values{"action": {"review_preview"}, "approval_id": {approvalRows[0].ID}, "decision": {"approved"}}
	reviewResponse := httptest.NewRecorder()
	review := httptest.NewRequest(http.MethodPost, "/component-gitops-previews", strings.NewReader(reviewForm.Encode()))
	review.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(reviewResponse, review)
	if reviewResponse.Code != http.StatusSeeOther {
		t.Fatalf("review status=%d body=%s", reviewResponse.Code, reviewResponse.Body.String())
	}
}

func TestGitOpsPreviewRejectsConnectionForDifferentGroup(t *testing.T) {
	requests := memory.NewComponentLifecycleRequestStore()
	plans := memory.NewComponentLifecycleExecutionStore()
	previews := memory.NewComponentGitOpsPreviewStore()
	previewApprovals := memory.NewGitOpsPreviewApprovalStore()
	connections := memory.NewIntegrationConnectionStore()
	request, _ := lifecycle.NewRequest(lifecycle.Spec{Operation: lifecycle.Install, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", DesiredVersion: "0.149.0", Reason: "new gateway"}, "operator")
	_ = requests.Create(t.Context(), request)
	plan := &lifecycle.ExecutionPlan{ID: "plan", RequestID: request.ID, RequestSpecHash: request.SpecHash, ExecutorKind: lifecycle.ExecutorGitOps, PlanHash: "plan-hash"}
	_ = plans.Create(t.Context(), plan)
	connection, _ := integrations.NewConnection(integrations.Connection{Name: "orders", Provider: integrations.GitLab, Organization: "acme", Repository: "telemetry", Branch: "main", AllowedRoot: "fleetamp/groups", Mode: integrations.ModePullRequest, CredentialRef: "secret://gitlab/prod", GroupIDs: []string{"orders"}, Enabled: true}, "admin")
	_ = connections.Create(t.Context(), connection)
	mux := http.NewServeMux()
	registerGitOpsPreviewRoutes(mux, plans, requests, previews, previewApprovals, connections, nil, nil)
	form := url.Values{"plan_id": {"plan"}, "connection_id": {connection.ID}}
	response := httptest.NewRecorder()
	post := httptest.NewRequest(http.MethodPost, "/component-gitops-previews", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, post)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
