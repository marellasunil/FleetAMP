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
