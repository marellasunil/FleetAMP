// SQLite persistence backend for FleetAMP configuration state.
//
// Purpose:
//
//	Opens the embedded FleetAMP database and owns schema initialization for
//	configuration artifacts and per-agent assignments.
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

func (d *Database) Close() error                  { return d.db.Close() }
func (d *Database) Configurations() *ConfigStore  { return &ConfigStore{db: d.db} }
func (d *Database) Assignments() *AssignmentStore { return &AssignmentStore{db: d.db} }

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
	}
	for _, statement := range statements {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}
	return d.db.PingContext(ctx)
}
