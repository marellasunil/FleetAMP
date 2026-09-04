package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type listenerTLSConfig struct {
	Enabled           bool
	CertFile          string
	KeyFile           string
	ClientCAFile      string
	RequireClientCert bool
	Config            *tls.Config
}

type transportTLSConfig struct {
	HTTP  listenerTLSConfig
	OpAMP listenerTLSConfig
}

// loadTransportTLSConfig builds independent TLS settings for the web and OpAMP listeners.
func loadTransportTLSConfig() (transportTLSConfig, error) {
	httpTLS, err := loadListenerTLSConfig("FLEETAMP_HTTP_TLS", false)
	if err != nil {
		return transportTLSConfig{}, err
	}
	opampTLS, err := loadListenerTLSConfig("FLEETAMP_OPAMP_TLS", true)
	if err != nil {
		return transportTLSConfig{}, err
	}
	return transportTLSConfig{HTTP: httpTLS, OpAMP: opampTLS}, nil
}

// loadListenerTLSConfig validates one listener's certificate, key, protocol floor, and optional client CA.
func loadListenerTLSConfig(prefix string, allowClientAuth bool) (listenerTLSConfig, error) {
	certFile := strings.TrimSpace(os.Getenv(prefix + "_CERT_FILE"))
	keyFile := strings.TrimSpace(os.Getenv(prefix + "_KEY_FILE"))
	clientCAFile := strings.TrimSpace(os.Getenv(prefix + "_CLIENT_CA_FILE"))
	cfg := listenerTLSConfig{
		Enabled:           boolEnv(prefix + "_ENABLED"),
		CertFile:          certFile,
		KeyFile:           keyFile,
		ClientCAFile:      clientCAFile,
		RequireClientCert: boolEnv(prefix + "_REQUIRE_CLIENT_CERT"),
	}
	if !cfg.Enabled {
		if certFile != "" || keyFile != "" || clientCAFile != "" || cfg.RequireClientCert {
			return cfg, fmt.Errorf("%s_ENABLED must be true when TLS options are configured", prefix)
		}
		return cfg, nil
	}
	tlsDir, err := defaultTLSDirectory()
	if err != nil {
		return cfg, err
	}
	if cfg.CertFile == "" {
		cfg.CertFile = filepath.Join(tlsDir, "server.crt")
	}
	if cfg.KeyFile == "" {
		cfg.KeyFile = filepath.Join(tlsDir, "server.key")
	}
	if cfg.RequireClientCert && cfg.ClientCAFile == "" {
		cfg.ClientCAFile = filepath.Join(tlsDir, "agent-ca.crt")
	}
	certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return cfg, fmt.Errorf("load %s certificate: %w", prefix, err)
	}
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}
	if cfg.ClientCAFile != "" {
		if !allowClientAuth {
			return cfg, fmt.Errorf("%s_CLIENT_CA_FILE is not supported for the web listener", prefix)
		}
		pem, err := os.ReadFile(cfg.ClientCAFile)
		if err != nil {
			return cfg, fmt.Errorf("read %s client CA: %w", prefix, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return cfg, fmt.Errorf("%s_CLIENT_CA_FILE contains no valid certificates", prefix)
		}
		tlsConfig.ClientCAs = pool
	}
	if cfg.RequireClientCert {
		if !allowClientAuth {
			return cfg, fmt.Errorf("%s_REQUIRE_CLIENT_CERT is not supported for the web listener", prefix)
		}
		if tlsConfig.ClientCAs == nil {
			return cfg, fmt.Errorf("%s_CLIENT_CA_FILE is required when client certificates are required", prefix)
		}
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}
	cfg.Config = tlsConfig
	return cfg, nil
}

// defaultTLSDirectory returns the customer override or the platform user configuration directory for FleetAMP certificates.
func defaultTLSDirectory() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("FLEETAMP_TLS_DIR")); configured != "" {
		return configured, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve default TLS directory: %w", err)
	}
	return filepath.Join(configDir, "fleetamp", "tls"), nil
}
