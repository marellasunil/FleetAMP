package sqlite

import (
	"context"
	"path/filepath"
	"testing"
)

func TestGroupMembershipRolesAreStoredPerGroup(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := db.Authentication()
	if err := store.Create(ctx, User{
		Username: "alice", Role: "group_owner", Enabled: true,
		GroupIDs:     []string{"payments-prod", "payments-test"},
		PasswordSalt: []byte("salt"), PasswordHash: []byte("hash"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.ReplaceMemberships(ctx, "alice", []GroupMembership{
		{Username: "alice", GroupID: "payments-prod", RoleIDs: []string{"group_owner", "deployment_approver"}},
		{Username: "alice", GroupID: "payments-test", RoleIDs: []string{"viewer"}},
	}); err != nil {
		t.Fatal(err)
	}
	memberships, err := store.ListMemberships(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 2 {
		t.Fatalf("memberships=%d, want 2", len(memberships))
	}
	if memberships[0].GroupID != "payments-prod" || len(memberships[0].RoleIDs) != 2 {
		t.Fatalf("unexpected production membership: %+v", memberships[0])
	}
	if memberships[1].GroupID != "payments-test" || len(memberships[1].RoleIDs) != 1 || memberships[1].RoleIDs[0] != "viewer" {
		t.Fatalf("unexpected test membership: %+v", memberships[1])
	}
	user, err := store.Get(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(user.GroupIDs) != 2 {
		t.Fatalf("compatibility group IDs=%v", user.GroupIDs)
	}
}

func TestRBACRoleCatalogueIsSeeded(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "fleetamp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	roles, err := db.Authentication().ListRBACRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != len(BuiltinRBACRoles) {
		t.Fatalf("roles=%d, want %d", len(roles), len(BuiltinRBACRoles))
	}
}
