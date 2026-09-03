package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func clearSecurityEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"FLEETAMP_AUTH_USERNAME", "FLEETAMP_AUTH_PASSWORD",
		"FLEETAMP_OPAMP_TOKEN", "FLEETAMP_ALLOWED_ORIGINS",
		"FLEETAMP_MAX_REQUEST_BODY_BYTES", "FLEETAMP_ALLOW_INSECURE",
		"FLEETAMP_HTTP_TLS_ENABLED", "FLEETAMP_HTTP_TLS_TERMINATED",
		"FLEETAMP_OPAMP_TLS_ENABLED", "FLEETAMP_OPAMP_TLS_TERMINATED",
		"FLEETAMP_HTTP_TLS_CERT_FILE", "FLEETAMP_HTTP_TLS_KEY_FILE",
		"FLEETAMP_OPAMP_TLS_CERT_FILE", "FLEETAMP_OPAMP_TLS_KEY_FILE",
		"FLEETAMP_OPAMP_TLS_CLIENT_CA_FILE", "FLEETAMP_OPAMP_TLS_REQUIRE_CLIENT_CERT",
	} {
		t.Setenv(name, "")
	}
}

func TestSecurityConfigAllowsLoopbackDefaults(t *testing.T) {
	clearSecurityEnv(t)
	cfg, err := loadSecurityConfig("127.0.0.1:8080", "localhost:4320")
	if err != nil {
		t.Fatalf("load security config: %v", err)
	}
	if cfg.MaxBodyBytes != defaultMaxRequestBodyBytes {
		t.Fatalf("max body=%d", cfg.MaxBodyBytes)
	}
}
func TestSecurityConfigRequiresRemoteOpAMPToken(t *testing.T) {
	clearSecurityEnv(t)
	t.Setenv("FLEETAMP_HTTP_TLS_TERMINATED", "true")
	if _, err := loadSecurityConfig(":8080", "127.0.0.1:4320"); err != nil {
		t.Fatalf("terminated TLS and bootstrap authentication should protect remote HTTP: %v", err)
	}

	t.Setenv("FLEETAMP_AUTH_USERNAME", "operator")
	t.Setenv("FLEETAMP_AUTH_PASSWORD", "a-strong-password")
	if _, err := loadSecurityConfig(":8080", ":4320"); err == nil {
		t.Fatal("expected remote OpAMP listener to require a token")
	}
}

func TestSecurityConfigAcceptsProtectedRemoteListeners(t *testing.T) {
	clearSecurityEnv(t)
	t.Setenv("FLEETAMP_AUTH_USERNAME", "operator")
	t.Setenv("FLEETAMP_AUTH_PASSWORD", "a-strong-password")
	t.Setenv("FLEETAMP_OPAMP_TOKEN", strings.Repeat("t", 32))
	t.Setenv("FLEETAMP_HTTP_TLS_TERMINATED", "true")
	t.Setenv("FLEETAMP_OPAMP_TLS_TERMINATED", "true")
	if _, err := loadSecurityConfig(":8080", ":4320"); err != nil {
		t.Fatalf("protected remote listeners rejected: %v", err)
	}
}

func TestSecurityConfigRejectsWeakSecrets(t *testing.T) {
	clearSecurityEnv(t)
	t.Setenv("FLEETAMP_AUTH_USERNAME", "operator")
	t.Setenv("FLEETAMP_AUTH_PASSWORD", "short")
	if _, err := loadSecurityConfig("127.0.0.1:8080", "127.0.0.1:4320"); err == nil {
		t.Fatal("expected weak password rejection")
	}
}
func TestSecurityMiddlewareAuthenticationAndHeaders(t *testing.T) {
	cfg := securityConfig{
		HTTPUsername: "operator",
		HTTPPassword: "a-strong-password",
		MaxBodyBytes: defaultMaxRequestBodyBytes,
	}
	handler := securityMiddleware(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/agents", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", unauthorized.Code)
	}
	if unauthorized.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing Content-Security-Policy")
	}

	authorizedRequest := httptest.NewRequest(http.MethodGet, "/agents", nil)
	authorizedRequest.SetBasicAuth("operator", "a-strong-password")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status=%d", authorized.Code)
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status=%d", health.Code)
	}
}
func TestSecurityMiddlewareRejectsCrossSiteMutation(t *testing.T) {
	cfg := securityConfig{MaxBodyBytes: defaultMaxRequestBodyBytes}
	handler := securityMiddleware(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "http://fleetamp.example/groups", nil)
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-site mutation status=%d", response.Code)
	}
}

func TestSecurityMiddlewareLimitsRequestBody(t *testing.T) {
	cfg := securityConfig{MaxBodyBytes: 1024}
	handler := securityMiddleware(cfg, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			http.Error(w, "request too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(strings.Repeat("x", 2048)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized request status=%d", response.Code)
	}
}

func TestValidRequestOriginAcceptsEquivalentLoopbackAliases(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/setup", nil)
	request.Host = "127.0.0.1:8080"
	request.Header.Set("Origin", "http://localhost:8080")
	if !validRequestOrigin(request, nil) {
		t.Fatal("equivalent loopback origin rejected")
	}

	request.Header.Set("Origin", "http://localhost:9090")
	if validRequestOrigin(request, nil) {
		t.Fatal("loopback origin with a different port accepted")
	}
}

func TestValidRequestOriginAllowsNullOnlyForSameSiteLoopback(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "http://localhost:8080/setup", nil)
	request.Host = "localhost:8080"
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Origin", "null")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	if !validRequestOrigin(request, nil) {
		t.Fatal("same-origin loopback null origin rejected")
	}

	request.RemoteAddr = "192.0.2.10:54321"
	if validRequestOrigin(request, nil) {
		t.Fatal("remote null origin accepted")
	}
	request.RemoteAddr = "127.0.0.1:54321"
	request.Host = "fleetamp.example.com"
	if validRequestOrigin(request, nil) {
		t.Fatal("non-loopback host with null origin accepted")
	}
}
