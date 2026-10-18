package main

import (
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDistributionBuildPlanCreatesImmutablePlanWithoutExecution(t *testing.T) {
	distributionStore := memory.NewDistributionStore()
	d, err := distributions.New("Platform Collector", "0.140.0", "v0.140.0", "", "", distributions.ComponentSet{Receivers: []string{"otlp"}}, "receivers:\n  otlp: {}", nil, []string{"receivers"}, "admin", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := distributionStore.Publish(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	planStore := memory.NewDistributionBuildPlanStore()
	mux := http.NewServeMux()
	registerDistributionBuildPlanRoutes(mux, distributionStore, planStore, memory.NewDistributionBuildApprovalStore(), memory.NewDistributionBuildExecutionStore(), memory.NewDistributionBuildPreflightStore(), memory.NewDistributionBuildWorkerResultStore(), nil)
	form := url.Values{"distribution_id": {d.ID}, "outputs": {"oci-image"}, "destination_ref": {"oci://registry.example/platform/collector:0.140.0"}, "credential_ref": {"secret://registry/platform"}}
	request := httptest.NewRequest(http.MethodPost, "/collector-distributions/build-plans", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	plans, err := planStore.List(t.Context(), 10)
	if err != nil || len(plans) != 1 {
		t.Fatalf("plans=%d err=%v", len(plans), err)
	}
	if plans[0].Status != distributions.BuildStatusPlanned {
		t.Fatalf("status=%q", plans[0].Status)
	}
}
