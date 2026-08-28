package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

func TestConfigurationAndAssignmentPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fleetamp.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	configuration := configs.NewConfiguration("test", "1", "service: {}\n", "text/yaml")
	if err := db.Configurations().Put(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	assignment := &configs.Assignment{AgentInstanceUID: "agent-1", ConfigurationID: configuration.ID, ConfigurationHash: configuration.Hash, Status: configs.DeliverySent, UpdatedAt: time.Now().UTC()}
	if err := db.Assignments().Upsert(ctx, assignment); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	gotConfig, err := reopened.Configurations().Get(ctx, configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotConfig.Hash != configuration.Hash || gotConfig.Content != configuration.Content {
		t.Fatalf("configuration mismatch: %#v", gotConfig)
	}
	gotAssignment, err := reopened.Assignments().Get(ctx, "agent-1", configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotAssignment.Status != configs.DeliverySent {
		t.Fatalf("status=%s", gotAssignment.Status)
	}
	if err := reopened.Assignments().UpdateByAgentHash(ctx, "agent-1", configuration.Hash, configs.DeliveryApplied, ""); err != nil {
		t.Fatal(err)
	}
	gotAssignment, err = reopened.Assignments().Get(ctx, "agent-1", configuration.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotAssignment.Status != configs.DeliveryApplied {
		t.Fatalf("status=%s", gotAssignment.Status)
	}
}
