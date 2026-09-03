package main

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const defaultMaxRequestBodyBytes int64 = 1 << 20

type securityConfig struct {
	HTTPUsername       string
	HTTPPassword       string
	OpAMPToken         string
	AllowedOrigins     map[string]struct{}
	MaxBodyBytes       int64
	AllowInsecure      bool
	HTTPTLSTerminated  bool
	OpAMPTLSTerminated bool
	HTTPNativeTLS      bool
	OpAMPNativeTLS     bool
}

func loadSecurityConfig(httpAddr, opampAddr string) (securityConfig, error) {
	cfg := securityConfig{
		HTTPUsername:       strings.TrimSpace(os.Getenv("FLEETAMP_AUTH_USERNAME")),
		HTTPPassword:       os.Getenv("FLEETAMP_AUTH_PASSWORD"),
		OpAMPToken:         os.Getenv("FLEETAMP_OPAMP_TOKEN"),
		AllowedOrigins:     parseAllowedOrigins(os.Getenv("FLEETAMP_ALLOWED_ORIGINS")),
		MaxBodyBytes:       defaultMaxRequestBodyBytes,
		AllowInsecure:      boolEnv("FLEETAMP_ALLOW_INSECURE"),
		HTTPTLSTerminated:  boolEnv("FLEETAMP_HTTP_TLS_TERMINATED"),
		OpAMPTLSTerminated: boolEnv("FLEETAMP_OPAMP_TLS_TERMINATED"),
		HTTPNativeTLS:      boolEnv("FLEETAMP_HTTP_TLS_ENABLED"),
		OpAMPNativeTLS:     boolEnv("FLEETAMP_OPAMP_TLS_ENABLED"),
	}
	if raw := strings.TrimSpace(os.Getenv("FLEETAMP_MAX_REQUEST_BODY_BYTES")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 1024 || value > 10<<20 {
			return cfg, fmt.Errorf("FLEETAMP_MAX_REQUEST_BODY_BYTES must be between 1024 and 10485760")
		}
		cfg.MaxBodyBytes = value
	}
	if (cfg.HTTPUsername == "") != (cfg.HTTPPassword == "") {
		return cfg, fmt.Errorf("FLEETAMP_AUTH_USERNAME and FLEETAMP_AUTH_PASSWORD must be configured together")
	}
	if cfg.HTTPPassword != "" && len(cfg.HTTPPassword) < 16 {
		return cfg, fmt.Errorf("FLEETAMP_AUTH_PASSWORD must contain at least 16 characters")
	}
	if cfg.OpAMPToken != "" && len(cfg.OpAMPToken) < 32 {
		return cfg, fmt.Errorf("FLEETAMP_OPAMP_TOKEN must contain at least 32 characters")
	}
	// Remote HTTP authentication is enforced by the bootstrap administrator and
	// server-pepper flow. Environment-based Basic Auth remains a migration fallback.
	if !isLoopbackListener(httpAddr) && !cfg.HTTPNativeTLS && !cfg.HTTPTLSTerminated && !cfg.AllowInsecure {
		return cfg, fmt.Errorf("remote HTTP listener %q requires native TLS, FLEETAMP_HTTP_TLS_TERMINATED=true, or explicit FLEETAMP_ALLOW_INSECURE=true", httpAddr)
	}
	if !isLoopbackListener(opampAddr) && cfg.OpAMPToken == "" && !cfg.AllowInsecure {
		return cfg, fmt.Errorf("remote OpAMP listener %q requires FLEETAMP_OPAMP_TOKEN or explicit FLEETAMP_ALLOW_INSECURE=true", opampAddr)
	}
	if !isLoopbackListener(opampAddr) && !cfg.OpAMPNativeTLS && !cfg.OpAMPTLSTerminated && !cfg.AllowInsecure {
		return cfg, fmt.Errorf("remote OpAMP listener %q requires native TLS, FLEETAMP_OPAMP_TLS_TERMINATED=true, or explicit FLEETAMP_ALLOW_INSECURE=true", opampAddr)
	}
	if cfg.AllowInsecure {
		slog.Warn("insecure remote listeners explicitly allowed", "component", "security", "event", "insecure_mode_enabled")
	}
	return cfg, nil
}

func boolEnv(name string) bool {
	value, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv(name)))
	return value
}

func isLoopbackListener(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
func parseAllowedOrigins(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		origin := strings.TrimRight(strings.TrimSpace(item), "/")
		if origin != "" {
			result[strings.ToLower(origin)] = struct{}{}
		}
	}
	return result
}

func securityMiddleware(cfg securityConfig, auth *authManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("HTTP handler panic recovered", "component", "http", "event", "panic_recovered")
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		if !allowedMethod(r.Method) {
			w.Header().Set("Allow", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		publicHealth := r.URL.Path == "/health" || r.URL.Path == "/ready"
		publicAuthentication := r.URL.Path == "/setup" || r.URL.Path == "/login"
		if !publicHealth && !publicAuthentication {
			if auth != nil && !auth.authorize(w, r, cfg) {
				return
			}
			if auth == nil && cfg.HTTPUsername != "" && !validBasicAuth(r, cfg) {
				w.Header().Set("WWW-Authenticate", `Basic realm="FleetAMP", charset="UTF-8"`)
				http.Error(w, "authentication required", http.StatusUnauthorized)
				return
			}
		}
		if isUnsafeMethod(r.Method) && !validRequestOrigin(r, cfg.AllowedOrigins) {
			slog.Warn("request origin rejected", "component", "http", "event", "origin_rejected", "origin", r.Header.Get("Origin"), "host", r.Host, "remote_address", r.RemoteAddr, "fetch_site", r.Header.Get("Sec-Fetch-Site"))
			http.Error(w, "request origin is not allowed", http.StatusForbidden)
			return
		}
		if isUnsafeMethod(r.Method) && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBodyBytes)
		}
		next.ServeHTTP(w, r)
	})
}
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Cache-Control", "no-store")
}

func allowedMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return true
	default:
		return false
	}
}

func isUnsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func validBasicAuth(r *http.Request, cfg securityConfig) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	expectedUser := sha256.Sum256([]byte(cfg.HTTPUsername))
	actualUser := sha256.Sum256([]byte(username))
	expectedPassword := sha256.Sum256([]byte(cfg.HTTPPassword))
	actualPassword := sha256.Sum256([]byte(password))
	return subtle.ConstantTimeCompare(actualUser[:], expectedUser[:]) == 1 &&
		subtle.ConstantTimeCompare(actualPassword[:], expectedPassword[:]) == 1
}
func internalServerError(w http.ResponseWriter, err error) {
	slog.Error("HTTP request failed", "component", "http", "event", "internal_error", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}

func validRequestOrigin(r *http.Request, allowed map[string]struct{}) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	if origin == "null" {
		return validLoopbackNullOrigin(r)
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	normalized := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
	if _, ok := allowed[normalized]; ok {
		return true
	}
	if strings.EqualFold(parsed.Host, r.Host) {
		return true
	}
	return equivalentLoopbackOrigin(parsed, r.Host)
}

func validLoopbackNullOrigin(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	remoteHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	fetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
	return loopbackHost(host) && loopbackHost(remoteHost) &&
		(fetchSite == "same-origin" || fetchSite == "same-site")
}

func equivalentLoopbackOrigin(origin *url.URL, requestHost string) bool {
	host, port, err := net.SplitHostPort(requestHost)
	if err != nil {
		host = requestHost
		port = ""
	}
	originPort := origin.Port()
	if originPort == "" {
		if origin.Scheme == "http" {
			originPort = "80"
		} else if origin.Scheme == "https" {
			originPort = "443"
		}
	}
	if port == "" {
		port = originPort
	}
	return loopbackHost(origin.Hostname()) && loopbackHost(host) && originPort == port
}

func loopbackHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	return strings.EqualFold(host, "localhost") || (net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
}
