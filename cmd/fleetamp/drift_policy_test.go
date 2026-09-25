package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

type fakeDriftPolicyStore struct{ policy configs.DriftPolicy }

func (s *fakeDriftPolicyStore) Get(context.Context) (configs.DriftPolicy, error) {
	return s.policy, nil
}

func (s *fakeDriftPolicyStore) Set(_ context.Context, policy configs.DriftPolicy) error {
	s.policy = policy
	return nil
}

func TestDriftPolicyPageAndUpdate(t *testing.T) {
	store := &fakeDriftPolicyStore{policy: configs.DriftPolicyReport}
	mux := http.NewServeMux()
	registerDriftPolicyRoutes(mux, store, &authManager{})

	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/settings/configuration-drift", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "Report only") {
		t.Fatalf("GET status=%d body=%q", get.Code, get.Body.String())
	}
	form := url.Values{"policy": {string(configs.DriftPolicyEnforce)}}
	post := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/settings/configuration-drift", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(post, request)
	if post.Code != http.StatusSeeOther {
		t.Fatalf("POST status=%d, want %d", post.Code, http.StatusSeeOther)
	}
	if store.policy != configs.DriftPolicyEnforce {
		t.Fatalf("stored policy=%q, want %q", store.policy, configs.DriftPolicyEnforce)
	}
}

func TestDriftPolicyRejectsInvalidValue(t *testing.T) {
	store := &fakeDriftPolicyStore{policy: configs.DriftPolicyReport}
	mux := http.NewServeMux()
	registerDriftPolicyRoutes(mux, store, &authManager{})

	form := url.Values{"policy": {"overwrite_anything"}}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/settings/configuration-drift", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", response.Code, http.StatusBadRequest)
	}
	if store.policy != configs.DriftPolicyReport {
		t.Fatalf("invalid request changed policy to %q", store.policy)
	}
}
