package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/audit"
)

type memoryAuditStore struct{ events []*audit.Event }

func (s *memoryAuditStore) Append(_ context.Context, event *audit.Event) error {
	s.events = append(s.events, event)
	return nil
}

func (s *memoryAuditStore) List(_ context.Context, filter audit.Filter) ([]*audit.Event, error) {
	return s.events, nil
}

func TestAuditMiddlewareRecordsFailedLoginWithoutRequestBody(t *testing.T) {
	store := &memoryAuditStore{}
	handler := auditMiddleware(&authManager{}, store, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
	}))
	form := url.Values{"username": {"sunil"}, "password": {"must-not-be-recorded"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if len(store.events) != 1 {
		t.Fatalf("events=%d, want 1", len(store.events))
	}
	event := store.events[0]
	if event.Actor != "sunil" || event.Action != "authentication.login" || event.Outcome != "denied" {
		t.Fatalf("unexpected event: %#v", event)
	}
	if strings.Contains(event.Path, "must-not-be-recorded") {
		t.Fatal("audit event exposed password")
	}
}
func TestAuditMiddlewareTreatsRenderedLoginFailureAsFailed(t *testing.T) {
	store := &memoryAuditStore{}
	handler := auditMiddleware(&authManager{}, store, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	form := url.Values{"username": {"sunil"}, "password": {"invalid"}}
	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), request)
	if len(store.events) != 1 || store.events[0].Outcome != "failed" {
		t.Fatalf("events=%#v", store.events)
	}
}

func TestAuditLogRendersAndRejectsInvalidDate(t *testing.T) {
	store := &memoryAuditStore{events: []*audit.Event{{
		Actor: "admin", Action: "policy.drift_update", ResourceType: "configuration_policy",
		Outcome: "success", HTTPMethod: "POST", Path: "/settings/configuration-drift", StatusCode: 303,
	}}}
	mux := http.NewServeMux()
	registerAuditRoutes(mux, store)

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/audit-log", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "policy.drift_update") {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}

	invalid := httptest.NewRecorder()
	mux.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/audit-log?from=invalid", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid date status=%d", invalid.Code)
	}
}

func TestDescribeAuditAction(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/groups/group-1", strings.NewReader("action=approve_deployment"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	action, resourceType, resourceID := describeAuditAction(request)
	if action != "deployment.approve_deployment" || resourceType != "group" || resourceID != "group-1" {
		t.Fatalf("descriptor=%q %q %q", action, resourceType, resourceID)
	}
}

func TestDescribeMembershipAuditAction(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/groups/group-1/collectors", strings.NewReader("action=approve&request_id=change-1"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	action, resource, id := describeAuditAction(request)
	if action != "membership.approve" || resource != "membership_change" || id != "change-1" {
		t.Fatalf("descriptor=%q %q %q", action, resource, id)
	}
}

func TestDescribeGitOpsAndIntegrationAuditActions(t *testing.T) {
	tests := []struct{ path, form, action, resource, id string }{
		{"/settings/integrations", "action=test_connection&connection_id=connection-1", "integration.validate", "integration_connection", "connection-1"},
		{"/component-gitops-previews", "action=queue_execution&approval_id=approval-1", "gitops.queue_execution", "gitops_preview", "approval-1"},
		{"/component-gitops-executions", "action=retry&execution_id=execution-1", "gitops.execution_retry", "gitops_execution", "execution-1"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.form))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		action, resource, id := describeAuditAction(request)
		if action != test.action || resource != test.resource || id != test.id {
			t.Errorf("%s: got %q %q %q", test.path, action, resource, id)
		}
		if !auditCategoryMatches("gitops", action) {
			t.Errorf("%s is missing from GitOps audit category", action)
		}
	}
}

func TestDeploymentAuditDetailsRecordsReviewerAndReason(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/groups/group-1", strings.NewReader(
		"action=request_deployment&assigned_reviewer=owner-one&change_reason=Enable+the+validated+metrics+pipeline"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	details := deploymentAuditDetails(request)
	if !strings.Contains(details, "reviewer=owner-one") || !strings.Contains(details, "Enable the validated metrics pipeline") {
		t.Fatalf("details=%q", details)
	}
}
