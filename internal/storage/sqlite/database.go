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
	"strings"

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

// DestinationProfiles returns administrator-controlled exporter destinations.
func (d *Database) DestinationProfiles() *DestinationProfileStore {
	return &DestinationProfileStore{db: d.db}
}

// Authentication returns the SQLite-backed user repository.
func (d *Database) Authentication() *AuthStore { return &AuthStore{db: d.db} }

// SectionPolicies returns the SQLite-backed configuration-section policy repository.
func (d *Database) SectionPolicies() *SectionPolicyStore { return &SectionPolicyStore{db: d.db} }

// DriftPolicy returns the SQLite-backed global drift policy repository.
func (d *Database) DriftPolicy() *DriftPolicyStore { return &DriftPolicyStore{db: d.db} }

// Audit returns the SQLite-backed append-only audit repository.
func (d *Database) Audit() *AuditStore { return &AuditStore{db: d.db} }

func (d *Database) GuideCatalog() *GuideCatalogStore { return &GuideCatalogStore{db: d.db} }

func (d *Database) GroupSecrets() *GroupSecretStore { return &GroupSecretStore{db: d.db} }

// Migrations returns the append-only migration provenance repository.
func (d *Database) Migrations() *MigrationStore { return &MigrationStore{db: d.db} }

// ComponentLifecycleRequests returns the immutable component proposal repository.
func (d *Database) ComponentLifecycleRequests() *ComponentLifecycleRequestStore {
	return &ComponentLifecycleRequestStore{db: d.db}
}

// ComponentLifecycleValidations returns immutable target and compatibility snapshots.
func (d *Database) ComponentLifecycleValidations() *ComponentLifecycleValidationStore {
	return &ComponentLifecycleValidationStore{db: d.db}
}

func (d *Database) ComponentLifecycleApprovals() *ComponentLifecycleApprovalStore {
	return &ComponentLifecycleApprovalStore{db: d.db}
}
func (d *Database) ComponentLifecycleExecutions() *ComponentLifecycleExecutionStore{return &ComponentLifecycleExecutionStore{db:d.db}}

// initialize creates all required tables and indexes in an idempotent transaction.
func (d *Database) initialize(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS configurations (
			id TEXT PRIMARY KEY, group_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, version TEXT NOT NULL,
            content TEXT NOT NULL, content_type TEXT NOT NULL, hash TEXT NOT NULL,
            created_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_configurations_created_at ON configurations(created_at)`,
		`CREATE TABLE IF NOT EXISTS migration_history (
			id TEXT PRIMARY KEY, configuration_id TEXT NOT NULL UNIQUE,
			group_id TEXT NOT NULL, group_name TEXT NOT NULL, name TEXT NOT NULL, version TEXT NOT NULL,
			source TEXT NOT NULL, agent_uid TEXT NOT NULL DEFAULT '', pattern_id TEXT NOT NULL,
			content_hash TEXT NOT NULL, created_by TEXT NOT NULL, created_at TEXT NOT NULL,
			FOREIGN KEY(configuration_id) REFERENCES configurations(id),
			FOREIGN KEY(group_id) REFERENCES groups(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_migration_history_group_created ON migration_history(group_id,created_at DESC)`,
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
            previous_configuration_id TEXT NOT NULL DEFAULT '', approval_request_id TEXT NOT NULL DEFAULT '',
            rollback_started_at TEXT,
            action TEXT NOT NULL, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '',
            created_at TEXT NOT NULL, sent_at TEXT, applying_at TEXT, applied_at TEXT,
            failed_at TEXT, updated_at TEXT NOT NULL,
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_agent_created ON deployments(agent_instance_uid, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_agent_hash ON deployments(agent_instance_uid, configuration_hash, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS groups (
            id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '',
            selector TEXT NOT NULL, owners TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name)`,
		`CREATE TABLE IF NOT EXISTS group_secrets (
			group_id TEXT NOT NULL, secret_key TEXT NOT NULL, ciphertext TEXT NOT NULL,
			updated_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			PRIMARY KEY(group_id,secret_key), FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS destination_profiles (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, environment TEXT NOT NULL,
			exporter_id TEXT NOT NULL, exporter_config TEXT NOT NULL,
			owner TEXT NOT NULL DEFAULT 'Platform Team',
			visibility TEXT NOT NULL DEFAULT 'organization',
			group_ids TEXT NOT NULL DEFAULT '[]',
			enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(name,environment)
		)`,
		`CREATE TABLE IF NOT EXISTS blueprint_patterns (
			id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '',
			platform TEXT NOT NULL, receiver_id TEXT NOT NULL, receiver_config TEXT NOT NULL,
			signals TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS blueprint_blocks (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL, component_id TEXT NOT NULL, config_yaml TEXT NOT NULL,
			signals TEXT NOT NULL DEFAULT '[]', platforms TEXT NOT NULL DEFAULT '[]',
			required INTEGER NOT NULL DEFAULT 0, locked INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(kind,component_id)
		)`,
		`CREATE TABLE IF NOT EXISTS group_deployment_requests (
            id TEXT PRIMARY KEY, group_id TEXT NOT NULL, group_name TEXT NOT NULL,
            group_selector TEXT NOT NULL, label_selector TEXT NOT NULL DEFAULT '{}', configuration_id TEXT NOT NULL,
            configuration_name TEXT NOT NULL, configuration_version TEXT NOT NULL,
            configuration_hash TEXT NOT NULL, base_configuration_id TEXT NOT NULL DEFAULT '',
            base_configuration_hash TEXT NOT NULL DEFAULT '', targets TEXT NOT NULL,
			requested_by TEXT NOT NULL, assigned_reviewer TEXT NOT NULL DEFAULT '',
			change_reason TEXT NOT NULL DEFAULT '', reviewed_by TEXT NOT NULL DEFAULT '',
            review_comment TEXT NOT NULL DEFAULT '', reviewed_at TEXT,
            expires_at TEXT NOT NULL, expired_at TEXT,
            status TEXT NOT NULL, created_at TEXT NOT NULL,
            FOREIGN KEY(group_id) REFERENCES groups(id),
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_group_deployment_requests_group_created ON group_deployment_requests(group_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_requests (
			id TEXT PRIMARY KEY, operation TEXT NOT NULL, component_type TEXT NOT NULL,
			group_id TEXT NOT NULL, label_selector TEXT NOT NULL DEFAULT '',
			deployment_method TEXT NOT NULL, current_version TEXT NOT NULL DEFAULT '',
			desired_version TEXT NOT NULL DEFAULT '', reason TEXT NOT NULL,
			spec_hash TEXT NOT NULL, requested_by TEXT NOT NULL,
			status TEXT NOT NULL, created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_created ON component_lifecycle_requests(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_group ON component_lifecycle_requests(group_id,created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_requests_no_update
			BEFORE UPDATE ON component_lifecycle_requests
			BEGIN SELECT RAISE(ABORT,'component lifecycle requests are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_requests_no_delete
			BEFORE DELETE ON component_lifecycle_requests
			BEGIN SELECT RAISE(ABORT,'component lifecycle requests are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_validations (
			id TEXT PRIMARY KEY, request_id TEXT NOT NULL, request_spec_hash TEXT NOT NULL,
			group_id TEXT NOT NULL, group_name TEXT NOT NULL, group_selector TEXT NOT NULL,
			label_selector TEXT NOT NULL, targets TEXT NOT NULL, findings TEXT NOT NULL,
			status TEXT NOT NULL, result_hash TEXT NOT NULL, validated_by TEXT NOT NULL, created_at TEXT NOT NULL,
			FOREIGN KEY(request_id) REFERENCES component_lifecycle_requests(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_validations_request ON component_lifecycle_validations(request_id,created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_validations_no_update
			BEFORE UPDATE ON component_lifecycle_validations
			BEGIN SELECT RAISE(ABORT,'component lifecycle validations are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_validations_no_delete
			BEFORE DELETE ON component_lifecycle_validations
			BEGIN SELECT RAISE(ABORT,'component lifecycle validations are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_approvals (
			id TEXT PRIMARY KEY, request_id TEXT NOT NULL, request_spec_hash TEXT NOT NULL,
			validation_id TEXT NOT NULL UNIQUE, validation_hash TEXT NOT NULL, group_id TEXT NOT NULL, group_name TEXT NOT NULL,
			operation TEXT NOT NULL, component_type TEXT NOT NULL, target_count INTEGER NOT NULL,
			requested_by TEXT NOT NULL, assigned_reviewer TEXT NOT NULL, submission_comment TEXT NOT NULL,
			status TEXT NOT NULL, reviewed_by TEXT NOT NULL DEFAULT '', review_comment TEXT NOT NULL DEFAULT '', reviewed_at TEXT, created_at TEXT NOT NULL,
			FOREIGN KEY(request_id) REFERENCES component_lifecycle_requests(id), FOREIGN KEY(validation_id) REFERENCES component_lifecycle_validations(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_approvals_status ON component_lifecycle_approvals(status,created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_execution_plans(
			id TEXT PRIMARY KEY,approval_id TEXT NOT NULL UNIQUE,request_id TEXT NOT NULL,request_spec_hash TEXT NOT NULL,
			validation_id TEXT NOT NULL,validation_hash TEXT NOT NULL,executor_kind TEXT NOT NULL,operation TEXT NOT NULL,
			component_type TEXT NOT NULL,deployment_method TEXT NOT NULL,targets TEXT NOT NULL,plan_hash TEXT NOT NULL,
			prepared_by TEXT NOT NULL,created_at TEXT NOT NULL,
			FOREIGN KEY(approval_id) REFERENCES component_lifecycle_approvals(id),FOREIGN KEY(request_id) REFERENCES component_lifecycle_requests(id),FOREIGN KEY(validation_id) REFERENCES component_lifecycle_validations(id))`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_execution_created ON component_lifecycle_execution_plans(created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_execution_no_update BEFORE UPDATE ON component_lifecycle_execution_plans BEGIN SELECT RAISE(ABORT,'component lifecycle execution plans are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_execution_no_delete BEFORE DELETE ON component_lifecycle_execution_plans BEGIN SELECT RAISE(ABORT,'component lifecycle execution plans are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS administrators (
            singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
            username TEXT NOT NULL UNIQUE, role TEXT NOT NULL DEFAULT 'admin', password_salt BLOB NOT NULL,
            password_hash BLOB NOT NULL, created_at TEXT NOT NULL,
            password_changed_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS users (
            username TEXT PRIMARY KEY COLLATE NOCASE,
            email TEXT NOT NULL DEFAULT '',
            role TEXT NOT NULL CHECK (role IN ('admin','operator','group_owner','viewer')),
			enabled INTEGER NOT NULL DEFAULT 1,
			group_ids TEXT NOT NULL DEFAULT '[]',
			timezone TEXT NOT NULL DEFAULT 'UTC',
			last_login_at TEXT,
			must_change_password INTEGER NOT NULL DEFAULT 0,
            password_salt BLOB NOT NULL, password_hash BLOB NOT NULL,
            created_at TEXT NOT NULL, password_changed_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_users_role_enabled ON users(role, enabled)`,
		`CREATE TABLE IF NOT EXISTS password_recovery_tokens (
			username TEXT PRIMARY KEY COLLATE NOCASE,
			token_hash BLOB NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
		)`,
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
		`CREATE TABLE IF NOT EXISTS audit_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            occurred_at TEXT NOT NULL, actor TEXT NOT NULL,
            action TEXT NOT NULL, resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '', outcome TEXT NOT NULL, details TEXT NOT NULL DEFAULT '',
            http_method TEXT NOT NULL, path TEXT NOT NULL, status_code INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_occurred ON audit_events(occurred_at DESC,id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_actor ON audit_events(actor,occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events(action,occurred_at DESC)`,
		`CREATE TABLE IF NOT EXISTS guide_catalog_versions (version INTEGER PRIMARY KEY, content TEXT NOT NULL, created_by TEXT NOT NULL, created_at TEXT NOT NULL, published_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS guide_catalog_state (singleton INTEGER PRIMARY KEY CHECK(singleton=1), active_version INTEGER NOT NULL, FOREIGN KEY(active_version) REFERENCES guide_catalog_versions(version))`,
		`CREATE TABLE IF NOT EXISTS guide_catalog_draft (singleton INTEGER PRIMARY KEY CHECK(singleton=1), content TEXT NOT NULL, base_version INTEGER NOT NULL, updated_by TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}
	if err := d.ensureGroupEnabledColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureConfigurationGroupColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureDestinationGovernanceColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureGroupOwnersColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureApprovalReviewColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureApprovalGovernanceColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureDeploymentRollbackColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureAdministratorRoleColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureGroupOwnerRole(ctx); err != nil {
		return err
	}
	if err := d.ensureUserEmailColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureUserProfileColumns(ctx); err != nil {
		return err
	}
	if err := d.ensurePasswordRecoverySchema(ctx); err != nil {
		return err
	}
	if err := d.ensureRBACSchema(ctx); err != nil {
		return err
	}
	if err := d.ensureAuditDetailsColumn(ctx); err != nil {
		return err
	}
	if err := d.migrateAdministratorToUsers(ctx); err != nil {
		return err
	}
	return d.db.PingContext(ctx)
}

func (d *Database) ensureDestinationGovernanceColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"owner", "TEXT NOT NULL DEFAULT 'Platform Team'"},
		{"visibility", "TEXT NOT NULL DEFAULT 'organization'"},
		{"group_ids", "TEXT NOT NULL DEFAULT '[]'"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "destination_profiles", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE destination_profiles ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add destination_profiles.%s column: %w", column.name, err)
		}
	}
	return nil
}

func (d *Database) ensureUserProfileColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"group_ids", "TEXT NOT NULL DEFAULT '[]'"},
		{"timezone", "TEXT NOT NULL DEFAULT 'UTC'"},
		{"last_login_at", "TEXT"},
		{"must_change_password", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "users", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add users.%s column: %w", column.name, err)
		}
	}
	return nil
}

func (d *Database) ensurePasswordRecoverySchema(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS password_recovery_tokens (
		username TEXT PRIMARY KEY COLLATE NOCASE,
		token_hash BLOB NOT NULL,
		expires_at TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
	)`)
	if err != nil {
		return fmt.Errorf("create password recovery token table: %w", err)
	}
	return nil
}

func (d *Database) ensureAuditDetailsColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "audit_events", "details")
	if err != nil || present {
		return err
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE audit_events ADD COLUMN details TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add audit_events.details column: %w", err)
	}
	return nil
}

func (d *Database) ensureConfigurationGroupColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "configurations", "group_id")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE configurations ADD COLUMN group_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add configurations.group_id column: %w", err)
	}
	return nil
}

func (d *Database) ensureUserEmailColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "users", "email")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add users.email column: %w", err)
	}
	return nil
}

// ensureGroupOwnerRole widens the users role constraint for databases created
// before resource-scoped group owners were introduced.
func (d *Database) ensureGroupOwnerRole(ctx context.Context) error {
	var schema string
	if err := d.db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&schema); err != nil {
		return fmt.Errorf("inspect users role constraint: %w", err)
	}
	if strings.Contains(schema, "group_owner") {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE users_next (
            username TEXT PRIMARY KEY COLLATE NOCASE,
            role TEXT NOT NULL CHECK (role IN ('admin','operator','group_owner','viewer')),
            enabled INTEGER NOT NULL DEFAULT 1,
            password_salt BLOB NOT NULL, password_hash BLOB NOT NULL,
            created_at TEXT NOT NULL, password_changed_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`INSERT INTO users_next SELECT username,role,enabled,password_salt,password_hash,created_at,password_changed_at,updated_at FROM users`,
		`DROP TABLE users`,
		`ALTER TABLE users_next RENAME TO users`,
		`CREATE INDEX idx_users_role_enabled ON users(role, enabled)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate users group_owner role: %w", err)
		}
	}
	return tx.Commit()
}

func (d *Database) ensureDeploymentRollbackColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"previous_configuration_id", "TEXT NOT NULL DEFAULT ''"},
		{"approval_request_id", "TEXT NOT NULL DEFAULT ''"},
		{"rollback_started_at", "TEXT"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "deployments", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE deployments ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add deployments.%s column: %w", column.name, err)
		}
	}
	return nil
}

func (d *Database) ensureApprovalReviewColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"label_selector", "TEXT NOT NULL DEFAULT '{}'"},
		{"base_configuration_id", "TEXT NOT NULL DEFAULT ''"},
		{"base_configuration_hash", "TEXT NOT NULL DEFAULT ''"},
		{"reviewed_by", "TEXT NOT NULL DEFAULT ''"},
		{"review_comment", "TEXT NOT NULL DEFAULT ''"},
		{"reviewed_at", "TEXT"},
		{"expires_at", "TEXT NOT NULL DEFAULT '9999-12-31T23:59:59Z'"},
		{"expired_at", "TEXT"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "group_deployment_requests", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE group_deployment_requests ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add group_deployment_requests.%s column: %w", column.name, err)
		}
	}
	if _, err := d.db.ExecContext(ctx, `UPDATE group_deployment_requests
        SET expires_at=strftime('%Y-%m-%dT%H:%M:%fZ',created_at,'+30 days')
        WHERE expires_at='9999-12-31T23:59:59Z'`); err != nil {
		return fmt.Errorf("backfill approval expiration: %w", err)
	}
	return nil
}

func (d *Database) ensureApprovalGovernanceColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"assigned_reviewer", "TEXT NOT NULL DEFAULT ''"},
		{"change_reason", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "group_deployment_requests", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE group_deployment_requests ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add group_deployment_requests.%s column: %w", column.name, err)
		}
	}
	return nil
}

func sqliteColumnExists(ctx context.Context, db *sql.DB, table, wanted string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == wanted {
			return true, nil
		}
	}
	return false, rows.Err()
}

// ensureGroupOwnersColumn migrates existing installations to resource-scoped group ownership.
func (d *Database) ensureGroupOwnersColumn(ctx context.Context) error {
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
		if name == "owners" {
			return nil
		}
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE groups ADD COLUMN owners TEXT NOT NULL DEFAULT '[]'`); err != nil {
		return fmt.Errorf("add groups.owners column: %w", err)
	}
	return nil
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
