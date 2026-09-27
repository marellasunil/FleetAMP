package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSideNavigationIsGroupedByProductArea(t *testing.T) {
	labels := []string{"Management", "Instrumentation", "Observe", "Administration"}
	last := -1
	for _, label := range labels {
		index := strings.Index(sideNav, ">"+label+"<")
		if index < 0 {
			t.Fatalf("side navigation is missing %q", label)
		}
		if index <= last {
			t.Fatalf("side navigation section %q is out of order", label)
		}
		last = index
	}

	for _, path := range []string{"/agents", "/groups", "/deployments", "/approvals", "/instrumentation", "/blueprints", "/pipelines", "/audit-log"} {
		if !strings.Contains(sideNav, `href="`+path+`"`) {
			t.Fatalf("side navigation is missing %q", path)
		}
	}
}

func TestInstrumentationGuideUpcomingPage(t *testing.T) {
	mux := http.NewServeMux()
	registerUIRoutes(mux)

	request := httptest.NewRequest(http.MethodGet, "/instrumentation", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, expected := range []string{"Instrumentation Guides", "Technology-specific OpenTelemetry onboarding", "Upcoming"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("instrumentation page is missing %q", expected)
		}
	}
}
