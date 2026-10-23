package main

import (
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDistributionCatalogPublishesAndRenders(t *testing.T) {
	store := memory.NewDistributionStore()
	releaseStore := memory.NewDistributionReleaseStore()
	mux := http.NewServeMux()
	registerDistributionRoutes(mux, store, releaseStore, configs.NewValidator(""), nil)
	form := url.Values{"name": {"Platform Collector"}, "version": {"0.140.0"}, "source_ref": {"v0.140.0"}, "receivers": {"otlp"}, "processors": {"batch"}, "exporters": {"otlphttp"}, "section_receivers": {"editable"}, "baseline_config": {`receivers:
  otlp:
    protocols:
      grpc: {}
exporters:
  otlphttp:
    endpoint: https://telemetry.example
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [otlphttp]`}}
	request := httptest.NewRequest(http.MethodPost, "/collector-distributions", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("POST status=%d body=%s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/collector-distributions", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status=%d", response.Code)
	}
	items, err := store.List(request.Context())
	if err != nil || len(items) != 1 {
		t.Fatalf("unexpected distributions: %#v %v", items, err)
	}
	if err = releaseStore.Create(request.Context(), &distributions.DistributionRelease{ID: "release-1", DistributionID: items[0].ID, ArtifactURI: "oci://registry.example/platform:0.140.0", ArtifactSHA256: strings.Repeat("a", 64), EvidenceHash: strings.Repeat("b", 64), ReleasedAt: time.Unix(2, 0)}); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/collector-distributions", nil))
	if !strings.Contains(response.Body.String(), "Released package") || !strings.Contains(response.Body.String(), "oci://registry.example/platform:0.140.0") {
		t.Fatalf("released package missing: %s", response.Body.String())
	}
	for _, want := range []string{"Platform Collector", "Collector 0.140.0", "Group-editable sections", "receivers", "Specification:", "Admin baseline", "SHA-256:"} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("page missing %q", want)
		}
	}
}
