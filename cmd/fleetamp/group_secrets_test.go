package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	groupsecrets "github.com/marellasunil/FleetAMP/internal/secrets"
	"github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func testGroupSecretService(t *testing.T) (*groupSecretService, string) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "secrets.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	group, err := groups.New("Payments API NL Prod", "", map[string]string{"service": "payments"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Groups().Create(ctx, group); err != nil {
		t.Fatal(err)
	}
	return &groupSecretService{store: db.GroupSecrets(), pepper: []byte("0123456789abcdef0123456789abcdef")}, group.ID
}

func putTestGroupSecret(t *testing.T, service *groupSecretService, groupID, key, value string) {
	t.Helper()
	ciphertext, err := service.encrypt(value)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := service.store.Upsert(context.Background(), &groupsecrets.GroupSecret{
		GroupID: groupID, Key: key, Ciphertext: ciphertext, UpdatedBy: "owner@example.com", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestGroupSecretMaterializeAndNormalize(t *testing.T) {
	service, groupID := testGroupSecretService(t)
	putTestGroupSecret(t, service, groupID, "otlp_token", "highly-sensitive-token")

	stored, err := service.store.Get(t.Context(), groupID, "otlp_token")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.Ciphertext, "highly-sensitive-token") {
		t.Fatal("secret was persisted in clear text")
	}

	configured := "headers:\n  Authorization: \"Bearer ${secret:otlp_token}\"\n"
	resolved, err := service.materialize(t.Context(), groupID, configured)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resolved, "Bearer highly-sensitive-token") || strings.Contains(resolved, "${secret:") {
		t.Fatalf("secret reference was not resolved: %s", resolved)
	}
	normalized, err := service.normalize(t.Context(), groupID, resolved)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(normalized, "highly-sensitive-token") || !strings.Contains(normalized, "<secret:otlp_token>") {
		t.Fatalf("secret was not safely normalized: %s", normalized)
	}
	if _, err := service.materialize(t.Context(), groupID, "token: ${secret:missing}"); err == nil {
		t.Fatal("expected missing secret reference to fail")
	}
	if err := service.validateReferences(t.Context(), groupID, configured); err != nil {
		t.Fatalf("configured reference did not validate: %v", err)
	}
	if err := service.validateReferences(t.Context(), groupID, "token: ${secret:missing}"); err == nil {
		t.Fatal("expected missing secret reference validation to fail")
	}
}

func TestConfigurationForDeliveryPreservesArtifactIdentity(t *testing.T) {
	service, groupID := testGroupSecretService(t)
	putTestGroupSecret(t, service, groupID, "token", "resolved-value")
	previous := runtimeGroupSecrets
	runtimeGroupSecrets = service
	t.Cleanup(func() { runtimeGroupSecrets = previous })

	configuration := configs.NewGroupConfiguration(groupID, "payments", "2.0.0", "token: ${secret:token}\n", "text/yaml")
	resolved, err := configurationForDelivery(t.Context(), configuration)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ID != configuration.ID || resolved.CreatedAt != configuration.CreatedAt || resolved.GroupID != configuration.GroupID {
		t.Fatalf("resolved configuration lost artifact identity: %#v", resolved)
	}
	if resolved.Hash == configuration.Hash || resolved.Content != "token: resolved-value\n" {
		t.Fatalf("resolved configuration did not produce delivery payload/hash: %#v", resolved)
	}
}

func TestSanitizeExporterPreview(t *testing.T) {
	preview, err := sanitizeExporterPreview("endpoint: https://otel.example.com\nheaders:\n  Authorization: plain-token\n  X-API-Key: ${secret:otlp_key}\ntls:\n  insecure: false\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"https://otel.example.com", "<protected>", "${secret:otlp_key}", "insecure: false"} {
		if !strings.Contains(preview, expected) {
			t.Fatalf("sanitized exporter preview is missing %q:\n%s", expected, preview)
		}
	}
	if strings.Contains(preview, "plain-token") {
		t.Fatalf("sanitized exporter preview leaked a token:\n%s", preview)
	}
}
