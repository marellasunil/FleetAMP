// SQLite persistence backend for FleetAMP configuration state.
//
// Purpose:
//
//	Opens the embedded FleetAMP database and owns schema initialization for
//	configuration artifacts, current assignments, and append-only deployment history.
//
// Packaging:
//
//	Uses modernc.org/sqlite, a CGo-free SQLite driver, so FleetAMP can ship as
//	a single Go binary without requiring a system SQLite installation.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Database struct{ db *sql.DB }

// Open creates the SQLite connection, applies safe connection settings, and initializes or migrates the schema.
func Open(ctx context.Context, path string) (*Database, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	database := &Database{db: db}
	if err := database.initialize(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return database, nil
}

// Close releases the underlying SQLite connection.
func (d *Database) Close() error { return d.db.Close() }

// Configurations returns the SQLite-backed configuration repository.
func (d *Database) Configurations() *ConfigStore { return &ConfigStore{db: d.db} }

// Assignments returns the SQLite-backed desired-state assignment repository.
func (d *Database) Assignments() *AssignmentStore { return &AssignmentStore{db: d.db} }

// Deployments returns the SQLite-backed delivery-history repository.
func (d *Database) Deployments() *DeploymentStore { return &DeploymentStore{db: d.db} }

// Groups returns the SQLite-backed group repository.
func (d *Database) Groups() *GroupStore { return &GroupStore{db: d.db} }

// Authentication returns the SQLite-backed singleton administrator repository.
func (d *Database) Authentication() *AuthStore { return &AuthStore{db: d.db} }

// initialize creates all required tables and indexes in an idempotent transaction.
func (d *Database) initialize(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS configurations (
            id TEXT PRIMARY KEY, name TEXT NOT NULL, version TEXT NOT NULL,
            content TEXT NOT NULL, content_type TEXT NOT NULL, hash TEXT NOT NULL,
            created_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_configurations_created_at ON configurations(created_at)`,
		`CREATE TABLE IF NOT EXISTS assignments (
            agent_instance_uid TEXT NOT NULL, configuration_id TEXT NOT NULL,
            configuration_hash TEXT NOT NULL, status TEXT NOT NULL,
            error TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL,
            PRIMARY KEY(agent_instance_uid, configuration_id),
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_agent_hash ON assignments(agent_instance_uid, configuration_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_updated_at ON assignments(updated_at)`,
		`CREATE TABLE IF NOT EXISTS deployments (
            id TEXT PRIMARY KEY, agent_instance_uid TEXT NOT NULL,
            configuration_id TEXT NOT NULL, configuration_name TEXT NOT NULL,
            configuration_version TEXT NOT NULL, configuration_hash TEXT NOT NULL,
            action TEXT NOT NULL, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '',
            created_at TEXT NOT NULL, sent_at TEXT, applying_at TEXT, applied_at TEXT,
            failed_at TEXT, updated_at TEXT NOT NULL,
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_agent_created ON deployments(agent_instance_uid, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_agent_hash ON deployments(agent_instance_uid, configuration_hash, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS groups (
            id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '',
            selector TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name)`,
		`CREATE TABLE IF NOT EXISTS administrators (
            singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
            username TEXT NOT NULL UNIQUE, password_salt BLOB NOT NULL,
            password_hash BLOB NOT NULL, created_at TEXT NOT NULL,
            password_changed_at TEXT NOT NULL
        )`,
	}
	for _, statement := range statements {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}
	if err := d.ensureGroupEnabledColumn(ctx); err != nil {
		return err
	}
	return d.db.PingContext(ctx)
}

// ensureGroupEnabledColumn migrates older group tables that predate the enabled flag.
func (d *Database) ensureGroupEnabledColumn(ctx context.Context) error {
	rows, err := d.db.QueryContext(ctx, `PRAGMA table_info(groups)`)
	if err != nil {
		return fmt.Errorf("inspect groups schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "enabled" {
			return nil
		}
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE groups ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1`); err != nil {
		return fmt.Errorf("add groups.enabled column: %w", err)
	}
	return nil
}
