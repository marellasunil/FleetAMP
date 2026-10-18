package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"github.com/marellasunil/FleetAMP/internal/storage/memory"
)

func TestDistributionCatalogPublishesAndRenders(t *testing.T) {
	store:=memory.NewDistributionStore();mux:=http.NewServeMux();registerDistributionRoutes(mux,store,nil)
	form:=url.Values{"name":{"Platform Collector"},"version":{"0.140.0"},"source_ref":{"v0.140.0"},"receivers":{"otlp"},"processors":{"batch"},"exporters":{"otlphttp"},"section_receivers":{"editable"}}
	request:=httptest.NewRequest(http.MethodPost,"/collector-distributions",strings.NewReader(form.Encode()));request.Header.Set("Content-Type","application/x-www-form-urlencoded");response:=httptest.NewRecorder();mux.ServeHTTP(response,request)
	if response.Code!=http.StatusSeeOther{t.Fatalf("POST status=%d body=%s",response.Code,response.Body.String())}
	response=httptest.NewRecorder();mux.ServeHTTP(response,httptest.NewRequest(http.MethodGet,"/collector-distributions",nil))
	if response.Code!=http.StatusOK{t.Fatalf("GET status=%d",response.Code)}
	for _,want:=range []string{"Platform Collector","Collector 0.140.0","Group-editable sections","receivers","Specification:"}{if !strings.Contains(response.Body.String(),want){t.Fatalf("page missing %q",want)}}
}
