// Tests SQLite schema persistence for configurations, assignments, deployments, and groups.
package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
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

func TestDeploymentHistoryPersistenceAndStatus(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "fleetamp.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	configuration := configs.NewConfiguration("gateway.yaml", "2", "service: {}\n", "text/yaml")
	if err := db.Configurations().Put(ctx, configuration); err != nil {
		t.Fatal(err)
	}

	first, err := configs.NewDeployment("agent-1", configuration, configs.DeploymentActionDeploy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Deployments().Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := configs.NewDeployment("agent-1", configuration, configs.DeploymentActionRollback)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Deployments().Create(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := db.Deployments().UpdateLatestByAgentHash(ctx, "agent-1", configuration.Hash, configs.DeliveryApplied, ""); err != nil {
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
	items, err := reopened.Deployments().ListByAgent(ctx, "agent-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("deployment count=%d", len(items))
	}
	if items[0].ID != second.ID || items[0].Action != configs.DeploymentActionRollback {
		t.Fatalf("latest deployment=%#v", items[0])
	}
	if items[0].Status != configs.DeliveryApplied || items[0].AppliedAt == nil {
		t.Fatalf("latest status=%#v", items[0])
	}
	if items[1].ID != first.ID || items[1].Status != configs.DeliveryPending {
		t.Fatalf("first deployment=%#v", items[1])
	}
}

func TestGroupPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "groups.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	group, err := groups.New("payments-prod", "Payments production", map[string]string{"team": "payments", "environment": "prod"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Groups().Create(ctx, group); err != nil {
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
	got, err := reopened.Groups().Get(ctx, group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != group.Name || got.Selector["team"] != "payments" || got.Selector["environment"] != "prod" {
		t.Fatalf("group mismatch: %#v", got)
	}
}
