// Tests SQLite schema persistence for configurations, assignments, deployments, and groups.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/groups"
)

func TestExistingAdministratorMigratesToAdminRole(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-auth.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`CREATE TABLE administrators (
		singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
		username TEXT NOT NULL UNIQUE, password_salt BLOB NOT NULL,
		password_hash BLOB NOT NULL, created_at TEXT NOT NULL,
		password_changed_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := legacy.Exec(`INSERT INTO administrators VALUES (1, ?, ?, ?, ?, ?)`,
		"existing-admin", []byte("0123456789abcdef"), []byte("hash"), now, now); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	admin, err := db.Authentication().Get(ctx, "existing-admin")
	if err != nil {
		t.Fatal(err)
	}
	if admin.Role != "admin" {
		t.Fatalf("migrated role=%q, want admin", admin.Role)
	}
	var legacyRows int
	if err := db.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM administrators").Scan(&legacyRows); err != nil {
		t.Fatal(err)
	}
	if legacyRows != 0 {
		t.Fatalf("legacy administrator credentials remain after migration: %d row(s)", legacyRows)
	}
}

func TestAdministratorDefaultsToAdminRole(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	admin := User{
		Username: "admin", Role: "admin", Enabled: true,
		PasswordSalt: []byte("0123456789abcdef"),
		PasswordHash: []byte("test-password-hash"),
	}
	if err := db.Authentication().Create(ctx, admin); err != nil {
		t.Fatal(err)
	}
	stored, err := db.Authentication().Get(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Role != "admin" {
		t.Fatalf("administrator role=%q, want admin", stored.Role)
	}
}

func TestUserRolesAndLastAdminProtection(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "users.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := db.Authentication()
	create := func(username, role string) {
		t.Helper()
		if err := store.Create(ctx, User{
			Username: username, Role: role, Enabled: true,
			PasswordSalt: []byte("0123456789abcdef"),
			PasswordHash: []byte("password-verifier"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	create("primary-admin", "admin")
	create("operator-one", "operator")
	create("owner-one", "group_owner")

	if err := store.UpdateRole(ctx, "primary-admin", "viewer"); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote last Admin error=%v", err)
	}
	if err := store.SetEnabled(ctx, "primary-admin", false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable last Admin error=%v", err)
	}
	if err := store.UpdateRole(ctx, "operator-one", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRole(ctx, "primary-admin", "viewer"); err != nil {
		t.Fatal(err)
	}
	users, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 3 || users[0].Username != "operator-one" || users[1].Role != "group_owner" || users[2].Role != "viewer" {
		t.Fatalf("unexpected users: %#v", users)
	}
}

func TestExistingUsersTableMigratesGroupOwnerRole(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy-users.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`CREATE TABLE users (
        username TEXT PRIMARY KEY COLLATE NOCASE,
        role TEXT NOT NULL CHECK (role IN ('admin','operator','viewer')),
        enabled INTEGER NOT NULL DEFAULT 1,
        password_salt BLOB NOT NULL, password_hash BLOB NOT NULL,
        created_at TEXT NOT NULL, password_changed_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
    )`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := legacy.Exec(`INSERT INTO users VALUES (?,?,?,?,?,?,?,?)`, "existing-admin", "admin", 1, []byte("salt"), []byte("hash"), now, now, now); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Authentication().Create(ctx, User{
		Username: "owner-one", Role: "group_owner", Enabled: true,
		PasswordSalt: []byte("salt"), PasswordHash: []byte("hash"),
	}); err != nil {
		t.Fatalf("create group owner after migration: %v", err)
	}
	owner, err := db.Authentication().Get(ctx, "owner-one")
	if err != nil || owner.Role != "group_owner" {
		t.Fatalf("owner=%#v err=%v", owner, err)
	}
}

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
	first.PreviousConfigurationID = "config-previous"
	first.ApprovalRequestID = "approval-1"
	if err := db.Deployments().Create(ctx, first); err != nil {
		t.Fatal(err)
	}
	claimed, err := db.Deployments().ClaimRollback(ctx, first.ID)
	if err != nil || !claimed {
		t.Fatalf("first rollback claim claimed=%t err=%v", claimed, err)
	}
	claimed, err = db.Deployments().ClaimRollback(ctx, first.ID)
	if err != nil || claimed {
		t.Fatalf("duplicate rollback claim claimed=%t err=%v", claimed, err)
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
	if items[1].ID != first.ID || items[1].Status != configs.DeliveryPending ||
		items[1].PreviousConfigurationID != "config-previous" || items[1].ApprovalRequestID != "approval-1" ||
		items[1].RollbackStartedAt == nil {
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
	group.Owners = []string{"operator-one", "operator-two"}
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
	if got.Name != group.Name || got.Selector["team"] != "payments" || got.Selector["environment"] != "prod" ||
		len(got.Owners) != 2 || got.Owners[0] != "operator-one" {
		t.Fatalf("group mismatch: %#v", got)
	}
}

func TestGroupDeploymentRequestPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "group-requests.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}

	group, err := groups.New("payments-prod", "Payments production", map[string]string{
		"application": "payments",
		"environment": "prod",
		"place":       "eu",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Groups().Create(ctx, group); err != nil {
		t.Fatal(err)
	}
	configuration := configs.NewConfiguration("collector.yaml", "3", "service: {}\n", "text/yaml")
	if err := db.Configurations().Put(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	request, err := configs.NewGroupDeploymentRequest(group.ID, group.Name, group.Selector, configuration, []configs.GroupDeploymentTarget{
		{AgentInstanceUID: "agent-ready", AgentName: "ready", Readiness: "Ready", Eligible: true},
		{AgentInstanceUID: "agent-offline", AgentName: "offline", Readiness: "Offline", Eligible: false},
	}, "admin")
	if err != nil {
		t.Fatal(err)
	}
	request.BaseConfigurationID = "config-baseline"
	request.BaseConfigurationHash = "baseline-hash"
	if err := db.GroupDeploymentRequests().Create(ctx, request); err != nil {
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
	items, err := reopened.GroupDeploymentRequests().ListByGroup(ctx, group.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("request count=%d", len(items))
	}
	got := items[0]
	if got.ID != request.ID || got.Status != configs.GroupDeploymentPendingApproval ||
		got.RequestedBy != "admin" || len(got.Targets) != 2 ||
		got.GroupSelector["application"] != "payments" ||
		got.ConfigurationHash != configuration.Hash || got.BaseConfigurationID != "config-baseline" ||
		got.BaseConfigurationHash != "baseline-hash" {
		t.Fatalf("request mismatch: %#v", got)
	}
	if err := reopened.GroupDeploymentRequests().Review(ctx, request.ID, configs.GroupDeploymentPendingApproval, configs.GroupDeploymentDeploying, "reviewer-admin", "approved after diff review"); err != nil {
		t.Fatal(err)
	}
	updated, err := reopened.GroupDeploymentRequests().Get(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != configs.GroupDeploymentDeploying || updated.ReviewedBy != "reviewer-admin" ||
		updated.ReviewComment != "approved after diff review" || updated.ReviewedAt == nil {
		t.Fatalf("updated status=%q", updated.Status)
	}
	if err := reopened.GroupDeploymentRequests().UpdateStatus(ctx, request.ID, configs.GroupDeploymentPendingApproval, configs.GroupDeploymentRejected); err == nil {
		t.Fatal("stale status transition unexpectedly succeeded")
	}
}

func TestAdministratorPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auth.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	admin := User{
		Username: "admin", Role: "admin", Enabled: true,
		PasswordSalt: []byte("0123456789abcdef"),
		PasswordHash: []byte("stored-password-verifier"),
	}
	if err := db.Authentication().Create(ctx, admin); err != nil {
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
	got, err := reopened.Authentication().Get(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != admin.Username ||
		string(got.PasswordHash) != string(admin.PasswordHash) {
		t.Fatalf("administrator mismatch: %#v", got)
	}
}

func TestConfigurationSectionPolicyPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "section-policies.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	store := db.SectionPolicies()
	policies, err := store.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var exportersEditable bool
	for _, policy := range policies {
		if policy.SectionKey == configs.SectionExporters {
			exportersEditable = policy.OperatorEditable
		}
	}
	if exportersEditable {
		t.Fatal("exporters should be read-only for Operators by default")
	}
	if err := store.SetOperatorEditable(ctx, configs.SectionExporters, true); err != nil {
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
	policies, err = reopened.SectionPolicies().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range policies {
		if policy.SectionKey == configs.SectionExporters {
			if !policy.OperatorEditable {
				t.Fatal("exporter policy did not persist")
			}
			return
		}
	}
	t.Fatal("exporter policy is missing")
}
