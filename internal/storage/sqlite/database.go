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

// GroupDeploymentRequests returns the approval-request repository.
func (d *Database) GroupDeploymentRequests() *GroupDeploymentRequestStore {
	return &GroupDeploymentRequestStore{db: d.db}
}

// Authentication returns the SQLite-backed user repository.
func (d *Database) Authentication() *AuthStore { return &AuthStore{db: d.db} }

// SectionPolicies returns the SQLite-backed configuration-section policy repository.
func (d *Database) SectionPolicies() *SectionPolicyStore { return &SectionPolicyStore{db: d.db} }

// DriftPolicy returns the SQLite-backed global drift policy repository.
func (d *Database) DriftPolicy() *DriftPolicyStore { return &DriftPolicyStore{db: d.db} }

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
		`CREATE TABLE IF NOT EXISTS group_deployment_requests (
            id TEXT PRIMARY KEY, group_id TEXT NOT NULL, group_name TEXT NOT NULL,
            group_selector TEXT NOT NULL, configuration_id TEXT NOT NULL,
            configuration_name TEXT NOT NULL, configuration_version TEXT NOT NULL,
            configuration_hash TEXT NOT NULL, targets TEXT NOT NULL,
            requested_by TEXT NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL,
            FOREIGN KEY(group_id) REFERENCES groups(id),
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_group_deployment_requests_group_created ON group_deployment_requests(group_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS administrators (
            singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
            username TEXT NOT NULL UNIQUE, role TEXT NOT NULL DEFAULT 'admin', password_salt BLOB NOT NULL,
            password_hash BLOB NOT NULL, created_at TEXT NOT NULL,
            password_changed_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS users (
            username TEXT PRIMARY KEY COLLATE NOCASE,
            role TEXT NOT NULL CHECK (role IN ('admin','operator','viewer')),
            enabled INTEGER NOT NULL DEFAULT 1,
            password_salt BLOB NOT NULL, password_hash BLOB NOT NULL,
            created_at TEXT NOT NULL, password_changed_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_users_role_enabled ON users(role, enabled)`,
		`CREATE TABLE IF NOT EXISTS configuration_section_policies (
            section_key TEXT PRIMARY KEY,
            operator_editable INTEGER NOT NULL CHECK (operator_editable IN (0, 1)),
            updated_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS configuration_drift_policy (
            singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
            policy TEXT NOT NULL CHECK (policy IN ('report_only','enforce')),
            updated_at TEXT NOT NULL
        )`,
		`INSERT OR IGNORE INTO configuration_drift_policy(singleton,policy,updated_at)
            VALUES(1,'report_only',CURRENT_TIMESTAMP)`,
	}
	for _, statement := range statements {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}
	if err := d.ensureGroupEnabledColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureAdministratorRoleColumn(ctx); err != nil {
		return err
	}
	if err := d.migrateAdministratorToUsers(ctx); err != nil {
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

// ensureAdministratorRoleColumn promotes the existing first-login account to
// Admin while making its authorization role explicit for RBAC-aware sessions.
func (d *Database) ensureAdministratorRoleColumn(ctx context.Context) error {
	rows, err := d.db.QueryContext(ctx, `PRAGMA table_info(administrators)`)
	if err != nil {
		return fmt.Errorf("inspect administrators schema: %w", err)
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
		if name == "role" {
			return nil
		}
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE administrators ADD COLUMN role TEXT NOT NULL DEFAULT 'admin'`); err != nil {
		return fmt.Errorf("add administrators.role column: %w", err)
	}
	return nil
}

// migrateAdministratorToUsers preserves the bootstrap account while moving
// authentication to the multi-user RBAC store. It is safe to run repeatedly.
func (d *Database) migrateAdministratorToUsers(ctx context.Context) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin administrator migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
        INSERT OR IGNORE INTO users (
            username, role, enabled, password_salt, password_hash,
            created_at, password_changed_at, updated_at
        )
        SELECT username, role, 1, password_salt, password_hash,
               created_at, password_changed_at, password_changed_at
        FROM administrators
    `); err != nil {
		return fmt.Errorf("migrate administrator to users: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM administrators`); err != nil {
		return fmt.Errorf("clear migrated administrator credential: %w", err)
	}
	return tx.Commit()
}
