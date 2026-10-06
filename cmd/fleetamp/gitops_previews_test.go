package main

import (
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
	request, _ := lifecycle.NewRequest(lifecycle.Spec{Operation: lifecycle.Upgrade, ComponentType: runtimes.OTelCollectorKubernetes, GroupID: "payments", DeploymentMethod: "gitops", CurrentVersion: "0.148.0", DesiredVersion: "0.149.0", Reason: "security"}, "operator")
	_ = requests.Create(t.Context(), request)
	plan := &lifecycle.ExecutionPlan{ID: "plan", RequestID: request.ID, RequestSpecHash: request.SpecHash, ExecutorKind: lifecycle.ExecutorGitOps, PlanHash: "plan-hash"}
	_ = plans.Create(t.Context(), plan)
	mux := http.NewServeMux()
	registerGitOpsPreviewRoutes(mux, plans, requests, previews, nil)
	form := url.Values{"plan_id": {"plan"}}
	response := httptest.NewRecorder()
	post := httptest.NewRequest(http.MethodPost, "/component-gitops-previews", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, post)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/component-gitops-previews", nil))
	for _, want := range []string{"GitOps proposal previews", "Preview SHA-256", "No repository, branch, commit, pull request, cluster or component was changed."} {
		if !strings.Contains(get.Body.String(), want) {
			t.Errorf("missing %q", want)
		}
	}
}
