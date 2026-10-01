package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInstrumentationGuideProvidesProgressiveDecisionMap(t *testing.T) {
	for _, expected := range []string{
		"Instrumentation Guides",
		"Application performance",
		"data-column=\"platform\" hidden",
		"data-column=\"technology\" hidden",
		"data-column=\"method\" hidden",
		"data-column=\"topology\" hidden",
		"Auto-instrumentation",
		"OpenTelemetry SDK",
		"Collector receiver",
		"eBPF / OBI",
		"OTel Operator",
		"DaemonSet",
		"Sidecar",
		"Central gateway",
		"Advantages",
		"Requirements",
		"Considerations",
	} {
		if !strings.Contains(guideHTML, expected) {
			t.Fatalf("Instrumentation Guide is missing %q", expected)
		}
	}
}

func TestInstrumentationGuideRevealsOneConnectedStageAtATime(t *testing.T) {
	for _, expected := range []string{
		`show("platform",!!goal)`,
		`show("technology",!!platform)`,
		`show("method",!!technology)`,
		`show("topology",!!method)`,
		`result.hidden=!topology`,
	} {
		if !strings.Contains(guideHTML+guideJS, expected) {
			t.Fatalf("progressive Guide behavior is missing %q", expected)
		}
	}
}


func TestInstrumentationGuideScriptAsset(t *testing.T) {
	mux := http.NewServeMux()
	registerGuideRoutes(mux)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/assets/guides.js", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET /assets/guides.js returned %d", response.Code)
	}
	if got := response.Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Fatalf("unexpected guides.js content type %q", got)
	}
	if !strings.Contains(response.Body.String(), "root.addEventListener") {
		t.Fatal("guides.js does not contain the interaction handler")
	}
}

func TestInstrumentationGuideRoute(t *testing.T) {
	mux := http.NewServeMux()
	registerGuideRoutes(mux)
	request := httptest.NewRequest(http.MethodGet, "/instrumentation", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /instrumentation returned %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "Instrumentation Guides") {
		t.Fatal("Guide route did not render the Guide page")
	}
}
