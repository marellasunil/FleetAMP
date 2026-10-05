package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/audit"
	sqlitestore "github.com/marellasunil/FleetAMP/internal/storage/sqlite"
)

type recoveryTokenOutput struct {
	Username  string `json:"username"`
	Token     string `json:"recovery_token"`
	ExpiresAt string `json:"expires_at"`
	Path      string `json:"recovery_path"`
}

// handleAdminCommand runs offline administrative commands before the server is initialized.
func handleAdminCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (bool, error) {
	if len(args) == 0 || args[0] != "admin" {
		return false, nil
	}
	if len(args) < 2 || args[1] != "recovery-token" {
		return true, fmt.Errorf("usage: fleetamp admin recovery-token --database PATH --username USERNAME")
	}
	flags := flag.NewFlagSet("admin recovery-token", flag.ContinueOnError)
	flags.SetOutput(stderr)
	databasePath := flags.String("database", "", "path to the FleetAMP SQLite database")
	username := flags.String("username", "", "account to recover")
	expiresIn := flags.Duration("expires", 15*time.Minute, "token lifetime")
	if err := flags.Parse(args[2:]); err != nil {
		return true, err
	}
	*databasePath, *username = strings.TrimSpace(*databasePath), strings.TrimSpace(*username)
	if *databasePath == "" || *username == "" {
		return true, fmt.Errorf("--database and --username are required")
	}
	if *expiresIn <= 0 || *expiresIn > time.Hour {
		return true, fmt.Errorf("--expires must be greater than zero and no more than 1h")
	}
	database, err := sqlitestore.Open(ctx, *databasePath)
	if err != nil {
		return true, err
	}
	defer database.Close()
	user, err := database.Authentication().Get(ctx, *username)
	if err != nil {
		return true, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return true, fmt.Errorf("generate recovery token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(token))
	expiresAt := time.Now().UTC().Add(*expiresIn)
	if err := database.Authentication().IssueRecoveryToken(ctx, user.Username, digest[:], expiresAt); err != nil {
		return true, err
	}
	if err := database.Audit().Append(ctx, &audit.Event{
		Actor: "local-admin-cli", Action: "authentication.recovery_token_issued",
		ResourceType: "user", ResourceID: user.Username, Outcome: "success",
		HTTPMethod: "CLI", Path: "admin recovery-token", StatusCode: 0,
		Details: "short-lived single-use recovery token issued",
	}); err != nil {
		return true, err
	}
	return true, json.NewEncoder(stdout).Encode(recoveryTokenOutput{
		Username: user.Username, Token: token, ExpiresAt: expiresAt.Format(time.RFC3339), Path: "/recover",
	})
}
