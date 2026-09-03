package opamp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdapterAuthorization(t *testing.T) {
	adapter := NewAdapter("127.0.0.1:4320", "01234567890123456789012345678901", nil)

	valid := httptest.NewRequest(http.MethodGet, "/v1/opamp", nil)
	valid.Header.Set("Authorization", "Bearer 01234567890123456789012345678901")
	if !adapter.authorized(valid) {
		t.Fatal("valid bearer token rejected")
	}

	missing := httptest.NewRequest(http.MethodGet, "/v1/opamp", nil)
	if adapter.authorized(missing) {
		t.Fatal("missing bearer token accepted")
	}

	wrong := httptest.NewRequest(http.MethodGet, "/v1/opamp", nil)
	wrong.Header.Set("Authorization", "Bearer wrong")
	if adapter.authorized(wrong) {
		t.Fatal("wrong bearer token accepted")
	}
}

func TestAdapterAuthorizationDisabled(t *testing.T) {
	adapter := NewAdapter("127.0.0.1:4320", "", nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/opamp", nil)
	if !adapter.authorized(request) {
		t.Fatal("request rejected when authentication is disabled")
	}
}
