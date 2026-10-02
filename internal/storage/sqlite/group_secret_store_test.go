package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/groups"
	groupsecrets "github.com/marellasunil/FleetAMP/internal/secrets"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

func TestGroupSecretStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "group-secrets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	group, err := groups.New("Payments API NL Prod", "", map[string]string{"service": "payments", "environment": "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Groups().Create(ctx, group); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC().Truncate(time.Nanosecond)
	item := &groupsecrets.GroupSecret{
		GroupID: group.ID, Key: "otlp/token", Ciphertext: "encrypted-one",
		UpdatedBy: "owner@example.com", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.GroupSecrets().Upsert(ctx, item); err != nil {
		t.Fatal(err)
	}
	stored, err := db.GroupSecrets().Get(ctx, group.ID, item.Key)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Ciphertext != "encrypted-one" || stored.UpdatedBy != item.UpdatedBy {
		t.Fatalf("unexpected stored secret metadata: %#v", stored)
	}

	item.Ciphertext = "encrypted-two"
	item.UpdatedBy = "admin@example.com"
	item.UpdatedAt = now.Add(time.Minute)
	if err := db.GroupSecrets().Upsert(ctx, item); err != nil {
		t.Fatal(err)
	}
	items, err := db.GroupSecrets().ListByGroup(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Ciphertext != "encrypted-two" || items[0].UpdatedBy != "admin@example.com" {
		t.Fatalf("unexpected group secrets: %#v", items)
	}

	if err := db.GroupSecrets().Delete(ctx, group.ID, item.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GroupSecrets().Get(ctx, group.ID, item.Key); !errors.Is(err, storage.ErrGroupSecretNotFound) {
		t.Fatalf("deleted secret error=%v, want ErrGroupSecretNotFound", err)
	}
}

