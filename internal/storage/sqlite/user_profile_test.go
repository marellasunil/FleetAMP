package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestUserProfileGroupsTimezoneAndLastLogin(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "profiles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := db.Authentication()
	if err := store.Create(ctx, User{Username: "owner", Role: "group_owner", Enabled: true, PasswordSalt: []byte("salt"), PasswordHash: []byte("hash"), GroupIDs: []string{"payments-nl-prod"}, Timezone: "Europe/Amsterdam"}); err != nil {
		t.Fatal(err)
	}
	login := time.Date(2026, 9, 29, 7, 30, 0, 0, time.UTC)
	if err := store.RecordLogin(ctx, "owner", login); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateGroups(ctx, "owner", []string{"payments-nl-prod", "checkout-eu-prod"}); err != nil {
		t.Fatal(err)
	}
	user, err := store.Get(ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	if user.Timezone != "Europe/Amsterdam" || len(user.GroupIDs) != 2 || user.LastLoginAt == nil || !user.LastLoginAt.Equal(login) {
		t.Fatalf("unexpected persisted profile: %#v", user)
	}
}
