package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSideNavigationIsGroupedByProductArea(t *testing.T) {
	labels := []string{"Management", "Instrumentation", "Intelligence", "Observe", "Administration"}
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

	for _, path := range []string{"/agents", "/groups", "/deployments", "/approvals", "/instrumentation", "/blueprints", "/ai-insights", "/mcp", "/pipelines", "/audit-log"} {
		if !strings.Contains(sideNav, `href="`+path+`"`) {
			t.Fatalf("side navigation is missing %q", path)
		}
	}
}

func TestIntelligenceUpcomingPagesDescribeGovernedRoadmap(t *testing.T) {
	mux := http.NewServeMux()
	registerUIRoutes(mux)

	tests := []struct {
		path     string
		expected []string
	}{
		{path: "/ai-insights", expected: []string{"AI Insights", "explainable remediation", "RBAC"}},
		{path: "/mcp", expected: []string{"MCP Server", "Model Context Protocol", "fully audited"}},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
		for _, expected := range test.expected {
			if !strings.Contains(response.Body.String(), expected) {
				t.Fatalf("%s is missing %q", test.path, expected)
			}
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
	for _, expected := range []string{"Instrumentation Guides", "Follow technology-specific OpenTelemetry onboarding", "Upcoming"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("instrumentation page is missing %q", expected)
		}
	}
}
