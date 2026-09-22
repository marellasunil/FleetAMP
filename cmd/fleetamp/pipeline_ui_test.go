package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestConfigurationPipelinePageRendersSafeOrderedFlow(t *testing.T) {
	content := `
receivers:
  otlp:
  hostmetrics:
processors:
  memory_limiter:
  batch:
  unused:
exporters:
  otlp/grafana:
    endpoint: example.invalid:4317
    headers:
      authorization: super-secret
service:
  pipelines:
    metrics:
      receivers: [hostmetrics, otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/grafana]
`
	configuration := configs.NewConfiguration("collector.yaml", "2.0.0", content, "text/yaml")
	store := memory.NewConfigStore()
	if err := store.Put(context.Background(), configuration); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerConfigurationUIRoutes(mux, store)

	request := httptest.NewRequest(http.MethodGet, "/configurations/"+configuration.ID, nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, want := range []string{
		"Pipeline visualization",
		"metrics telemetry",
		"hostmetrics",
		"memory_limiter",
		"batch",
		"otlp/grafana",
		`processor &#34;unused&#34; is defined but unused`,
		"Plain flow:",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	if strings.Contains(body, "super-secret") || strings.Contains(body, "example.invalid") {
		t.Fatal("page exposed sensitive component settings")
	}
	receiver := strings.Index(body, "hostmetrics")
	processor := strings.Index(body, "memory_limiter")
	exporter := strings.Index(body, "otlp/grafana")
	if receiver < 0 || processor <= receiver || exporter <= processor {
		t.Fatalf("pipeline order is incorrect: receiver=%d processor=%d exporter=%d", receiver, processor, exporter)
	}
}

func TestConfigurationPipelinePageReturnsNotFound(t *testing.T) {
	mux := http.NewServeMux()
	registerConfigurationUIRoutes(mux, memory.NewConfigStore())
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/configurations/missing", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}
