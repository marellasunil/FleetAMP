package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearTLSEnv(t *testing.T) {
	t.Helper()
	t.Setenv("FLEETAMP_TLS_DIR", "")
	for _, prefix := range []string{"FLEETAMP_HTTP_TLS", "FLEETAMP_OPAMP_TLS"} {
		for _, suffix := range []string{"_ENABLED", "_CERT_FILE", "_KEY_FILE", "_CLIENT_CA_FILE", "_REQUIRE_CLIENT_CERT"} {
			t.Setenv(prefix+suffix, "")
		}
	}
}

func TestTLSDisabledByDefault(t *testing.T) {
	clearTLSEnv(t)
	cfg, err := loadTransportTLSConfig()
	if err != nil {
		t.Fatalf("load TLS config: %v", err)
	}
	if cfg.HTTP.Enabled || cfg.OpAMP.Enabled {
		t.Fatal("TLS unexpectedly enabled")
	}
}

func TestTLSEnabledRequiresCertificateAndKey(t *testing.T) {
	clearTLSEnv(t)
	t.Setenv("FLEETAMP_HTTP_TLS_ENABLED", "true")
	if _, err := loadTransportTLSConfig(); err == nil || !strings.Contains(err.Error(), "server.crt") {
		t.Fatalf("expected missing default certificate error, got %v", err)
	}
}

func TestTLSRejectsOptionsWhileDisabled(t *testing.T) {
	clearTLSEnv(t)
	t.Setenv("FLEETAMP_OPAMP_TLS_CERT_FILE", filepath.Join(t.TempDir(), "server.crt"))
	if _, err := loadTransportTLSConfig(); err == nil {
		t.Fatal("expected disabled TLS options to be rejected")
	}
}

func TestTLSRejectsInvalidCertificate(t *testing.T) {
	clearTLSEnv(t)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	if err := os.WriteFile(certPath, []byte("not a certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("not a key"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLEETAMP_OPAMP_TLS_ENABLED", "true")
	t.Setenv("FLEETAMP_OPAMP_TLS_CERT_FILE", certPath)
	t.Setenv("FLEETAMP_OPAMP_TLS_KEY_FILE", keyPath)
	if _, err := loadTransportTLSConfig(); err == nil {
		t.Fatal("expected invalid key pair to be rejected")
	}
}
