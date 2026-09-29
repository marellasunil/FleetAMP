package main

import (
	"html/template"
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
	for _, expected := range []string{"Groups & Labels", "/account", "/assets/fleetamp-logo.svg"} {
		if !strings.Contains(sideNav, expected) {
			t.Fatalf("side navigation is missing %q", expected)
		}
	}
}

func TestFleetAMPLogoAsset(t *testing.T) {
	mux := http.NewServeMux()
	registerUIRoutes(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/fleetamp-logo.svg", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Header().Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("logo asset status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if !strings.Contains(response.Body.String(), "FleetAMP") {
		t.Fatal("logo asset is missing its accessible title")
	}
}

func TestAdministrationNavigationHighlightsOnePage(t *testing.T) {
	for _, page := range []string{"settings-users", "settings-sections", "settings-drift"} {
		var output strings.Builder
		tmpl := template.Must(template.New("navigation").Parse(sideNav))
		if err := tmpl.Execute(&output, struct{ Page string }{Page: page}); err != nil {
			t.Fatal(err)
		}
		if count := strings.Count(output.String(), "navitem active"); count != 1 {
			t.Fatalf("page %q highlighted %d navigation items", page, count)
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
