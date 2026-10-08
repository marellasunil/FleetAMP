package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestGitOpsExecutionMonitorRendersLatestStatusAndEvidence(t *testing.T) {
	requests := memory.NewComponentLifecycleRequestStore()
	plans := memory.NewComponentLifecycleExecutionStore()
	previews := memory.NewComponentGitOpsPreviewStore()
	executions := memory.NewGitOpsExecutionStore()

	request, _ := lifecycle.NewRequest(lifecycle.Spec{
		Operation:        lifecycle.Upgrade,
		ComponentType:    runtimes.OTelCollectorKubernetes,
		GroupID:          "payments",
		DeploymentMethod: "gitops",
		CurrentVersion:   "0.148.0",
		DesiredVersion:   "0.149.0",
		Reason:           "security",
	}, "operator")
	if err := requests.Create(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	plan := &lifecycle.ExecutionPlan{ID: "plan", RequestID: request.ID, RequestSpecHash: request.SpecHash, ExecutorKind: lifecycle.ExecutorGitOps, PlanHash: "plan-hash"}
	if err := plans.Create(t.Context(), plan); err != nil {
		t.Fatal(err)
	}
	preview := &lifecycle.GitOpsPreview{ID: "preview", PlanID: plan.ID, PlanHash: plan.PlanHash, PreviewHash: "preview-hash", RepositoryPath: "fleetamp/groups/payments/components/collector.yaml"}
	if err := previews.Create(t.Context(), preview); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	execution := &lifecycle.GitOpsExecutionRequest{ID: "execution", ApprovalID: "approval", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, PlanHash: plan.PlanHash, Provider: "github", Mode: "fleetamp-pull-request", RepositoryPath: preview.RepositoryPath, Branch: "main", RequestedBy: "admin", CreatedAt: now}
	queued := &lifecycle.GitOpsExecutionEvent{ID: "queued-event", ExecutionID: execution.ID, Status: lifecycle.GitOpsExecutionQueued, Actor: "admin", Message: "Queued", Evidence: map[string]string{"preview_hash": preview.PreviewHash}, CreatedAt: now}
	if err := executions.Create(t.Context(), execution, queued); err != nil {
		t.Fatal(err)
	}
	succeeded, err := lifecycle.NewGitOpsExecutionEvent(execution.ID, lifecycle.GitOpsExecutionSucceeded, "worker-1", "Dry-run provider validation succeeded", map[string]string{"adapter_mode": "dry-run", "repository_write": "false"})
	if err != nil {
		t.Fatal(err)
	}
	if err := executions.AppendEvent(t.Context(), succeeded); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerGitOpsExecutionRoutes(mux, executions, previews, plans, requests, nil, nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/component-gitops-executions?queued=execution", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for _, want := range []string{"GitOps execution monitor", "succeeded", "Dry-run provider validation succeeded", "adapter_mode=dry-run", "repository_write=false", "without writing to Git"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestGitOpsExecutionMonitorRejectsInvalidPost(t *testing.T) {
	mux := http.NewServeMux()
	registerGitOpsExecutionRoutes(mux, memory.NewGitOpsExecutionStore(), memory.NewComponentGitOpsPreviewStore(), memory.NewComponentLifecycleExecutionStore(), memory.NewComponentLifecycleRequestStore(), nil, nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/component-gitops-executions", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestGitOpsExecutionMonitorRetriesFailedExecution(t *testing.T) {
	requests := memory.NewComponentLifecycleRequestStore()
	plans := memory.NewComponentLifecycleExecutionStore()
	previews := memory.NewComponentGitOpsPreviewStore()
	executions := memory.NewGitOpsExecutionStore()
	request, _ := lifecycle.NewRequest(lifecycle.Spec{Operation: lifecycle.Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security"}, "operator")
	_ = requests.Create(t.Context(), request)
	plan := &lifecycle.ExecutionPlan{ID: "retry-plan", RequestID: request.ID, RequestSpecHash: request.SpecHash, ExecutorKind: lifecycle.ExecutorGitOps, PlanHash: "retry-plan-hash"}
	_ = plans.Create(t.Context(), plan)
	preview := &lifecycle.GitOpsPreview{ID: "retry-preview", PlanID: plan.ID, PlanHash: plan.PlanHash, PreviewHash: "retry-preview-hash", RepositoryPath: "fleetamp/groups/payments/components/collector.json"}
	_ = previews.Create(t.Context(), preview)
	execution := &lifecycle.GitOpsExecutionRequest{ID: "retry-execution", PreviewID: preview.ID, PreviewHash: preview.PreviewHash, PlanHash: plan.PlanHash, Provider: "github", Mode: "fleetamp-pull-request", RepositoryPath: preview.RepositoryPath, Branch: "main", RequestedBy: "admin", CreatedAt: time.Now().UTC()}
	queued := &lifecycle.GitOpsExecutionEvent{ID: "retry-queued", ExecutionID: execution.ID, Status: lifecycle.GitOpsExecutionQueued, Actor: "admin", Message: "queued", CreatedAt: time.Now().UTC()}
	_ = executions.Create(t.Context(), execution, queued)
	failed, _ := lifecycle.NewGitOpsExecutionEvent(execution.ID, lifecycle.GitOpsExecutionFailed, "worker", "provider unavailable", nil)
	_ = executions.AppendEvent(t.Context(), failed)

	mux := http.NewServeMux()
	registerGitOpsExecutionRoutes(mux, executions, previews, plans, requests, nil, nil)
	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/component-gitops-executions", nil))
	if !strings.Contains(get.Body.String(), "Retry failed execution") || !strings.Contains(get.Body.String(), "Attempt 2 of 3") {
		t.Fatalf("retry control missing: %s", get.Body.String())
	}
	form := url.Values{"action": {"retry"}, "execution_id": {execution.ID}, "reason": {"provider access restored"}}
	post := httptest.NewRequest(http.MethodPost, "/component-gitops-executions", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, post)
	if response.Code != http.StatusSeeOther || !strings.Contains(response.Header().Get("Location"), "retried=") {
		t.Fatalf("status=%d location=%q body=%s", response.Code, response.Header().Get("Location"), response.Body.String())
	}
	events, _ := executions.ListEvents(t.Context(), execution.ID)
	latest := events[len(events)-1]
	if latest.Status != lifecycle.GitOpsExecutionQueued || latest.Evidence["attempt"] != "2" || latest.Evidence["reason"] != "provider access restored" {
		t.Fatalf("latest=%#v", latest)
	}
}
