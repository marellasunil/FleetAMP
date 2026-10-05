package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/migrations"
)

func TestMigrationStoreAtomicallySavesConfigurationAndHistory(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "migration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	group, err := groups.New("Payments production", "", map[string]string{"team": "payments"})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Groups().Create(ctx, group); err != nil {
		t.Fatal(err)
	}
	configuration := configs.NewGroupConfiguration(group.ID, "Imported Collector", "migration-1", "receivers: {}\nservice:\n  pipelines: {}\n", "text/yaml")
	record := &migrations.Record{
		ID: configuration.ID, ConfigurationID: configuration.ID, GroupID: group.ID, GroupName: group.Name,
		Name: configuration.Name, Version: configuration.Version, Source: "existing-collector", AgentUID: "collector-1",
		PatternID: "custom", ContentHash: configuration.Hash, CreatedBy: "owner", CreatedAt: configuration.CreatedAt,
	}
	if err := database.Migrations().Save(ctx, configuration, record); err != nil {
		t.Fatal(err)
	}
	stored, err := database.Configurations().Get(ctx, configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GroupID != group.ID || stored.Hash != configuration.Hash {
		t.Fatalf("stored configuration=%#v", stored)
	}
	history, err := database.Migrations().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ConfigurationID != configuration.ID || history[0].CreatedBy != "owner" {
		t.Fatalf("history=%#v", history)
	}
}

func TestMigrationStoreRollsBackConfigurationWhenHistoryInsertFails(t *testing.T) {
	ctx := context.Background()
	database, err := Open(ctx, filepath.Join(t.TempDir(), "migration-rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	group, _ := groups.New("Payments", "", map[string]string{"team": "payments"})
	if err := database.Groups().Create(ctx, group); err != nil {
		t.Fatal(err)
	}
	first := configs.NewGroupConfiguration(group.ID, "Imported", "v1", "receivers: {}\n", "text/yaml")
	firstRecord := &migrations.Record{ID: "fixed-record", ConfigurationID: first.ID, GroupID: group.ID, GroupName: group.Name, Name: first.Name, Version: first.Version, Source: "other", PatternID: "custom", ContentHash: first.Hash, CreatedBy: "admin", CreatedAt: first.CreatedAt}
	if err := database.Migrations().Save(ctx, first, firstRecord); err != nil {
		t.Fatal(err)
	}
	second := configs.NewGroupConfiguration(group.ID, "Imported", "v2", "receivers: {otlp: {}}\n", "text/yaml")
	secondRecord := *firstRecord
	secondRecord.ConfigurationID, secondRecord.Version, secondRecord.ContentHash, secondRecord.CreatedAt = second.ID, second.Version, second.Hash, second.CreatedAt
	if err := database.Migrations().Save(ctx, second, &secondRecord); err == nil {
		t.Fatal("expected duplicate history ID to fail")
	}
	if _, err := database.Configurations().Get(ctx, second.ID); err == nil {
		t.Fatal("configuration insert was not rolled back")
	}
}
