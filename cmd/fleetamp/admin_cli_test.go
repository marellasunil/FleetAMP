package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

func TestRecoveryTokenCommandIssuesSingleUseToken(t *testing.T) {
	ctx := context.Background()
	databasePath := filepath.Join(t.TempDir(), "fleetamp.db")
	database, err := sqlitestore.Open(ctx, databasePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Authentication().Create(ctx, sqlitestore.User{
		Username: "admin", Role: "admin", Enabled: true,
		PasswordSalt: []byte("0123456789abcdef"), PasswordHash: []byte("verifier"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	handled, err := handleAdminCommand(ctx, []string{"admin", "recovery-token", "--database", databasePath, "--username", "admin"}, &stdout, &stderr)
	if err != nil || !handled {
		t.Fatalf("handled=%t err=%v stderr=%s", handled, err, stderr.String())
	}
	var result recoveryTokenOutput
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Token == "" || result.Username != "admin" || result.Path != "/recover" {
		t.Fatalf("unexpected output: %#v", result)
	}
}
