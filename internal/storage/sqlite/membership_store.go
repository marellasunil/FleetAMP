package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type GroupMembership struct {
	Username string
	GroupID  string
	RoleIDs  []string
}

type RBACRole struct {
	ID, Name, Scope, Description string
}

var BuiltinRBACRoles = []RBACRole{
	{ID: "group_owner", Name: "Group Owner", Scope: "group", Description: "Manage membership, configuration and deployments."},
	{ID: "configuration_editor", Name: "Configuration Editor", Scope: "group", Description: "Create, validate and submit configuration versions."},
	{ID: "deployment_approver", Name: "Deployment Approver", Scope: "group", Description: "Approve, reject or return deployment requests."},
	{ID: "deployment_operator", Name: "Deployment Operator", Scope: "group", Description: "Deploy approved versions and monitor rollback."},
	{ID: "auditor", Name: "Auditor", Scope: "group", Description: "Review audit, deployment and drift history."},
	{ID: "viewer", Name: "Viewer", Scope: "group", Description: "Read group resources without making changes."},
}

func (d *Database) ensureRBACSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS rbac_roles (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, scope TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '', builtin INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS group_memberships (
			username TEXT NOT NULL COLLATE NOCASE, group_id TEXT NOT NULL,
			created_at TEXT NOT NULL, PRIMARY KEY(username,group_id),
			FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS group_membership_roles (
			username TEXT NOT NULL COLLATE NOCASE, group_id TEXT NOT NULL, role_id TEXT NOT NULL,
			PRIMARY KEY(username,group_id,role_id),
			FOREIGN KEY(username,group_id) REFERENCES group_memberships(username,group_id) ON DELETE CASCADE,
			FOREIGN KEY(role_id) REFERENCES rbac_roles(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_group_memberships_group ON group_memberships(group_id,username)`,
	}
	for _, statement := range statements {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize RBAC schema: %w", err)
		}
	}
	for _, role := range BuiltinRBACRoles {
		if _, err := d.db.ExecContext(ctx, `INSERT OR IGNORE INTO rbac_roles(id,name,scope,description,builtin) VALUES(?,?,?,?,1)`,
			role.ID, role.Name, role.Scope, role.Description); err != nil {
			return fmt.Errorf("seed RBAC role %s: %w", role.ID, err)
		}
	}
	return d.migrateLegacyMemberships(ctx)
}
func (d *Database) migrateLegacyMemberships(ctx context.Context) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := d.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO group_memberships(username,group_id,created_at)
		SELECT users.username,json_each.value,?
		FROM users,json_each(users.group_ids)
		WHERE json_each.value <> ''
	`, now); err != nil {
		return fmt.Errorf("migrate group memberships: %w", err)
	}
	mappings := []struct{ legacy, role string }{
		{"group_owner", "group_owner"},
		{"operator", "configuration_editor"},
		{"operator", "deployment_operator"},
		{"viewer", "viewer"},
	}
	for _, mapping := range mappings {
		if _, err := d.db.ExecContext(ctx, `
			INSERT OR IGNORE INTO group_membership_roles(username,group_id,role_id)
			SELECT membership.username,membership.group_id,?
			FROM group_memberships membership
			JOIN users ON users.username=membership.username COLLATE NOCASE
			WHERE users.role=?
		`, mapping.role, mapping.legacy); err != nil {
			return fmt.Errorf("migrate role %s: %w", mapping.role, err)
		}
	}
	return nil
}

func (s *AuthStore) ListMemberships(ctx context.Context) ([]GroupMembership, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT membership.username,membership.group_id,role.role_id
		FROM group_memberships membership
		LEFT JOIN group_membership_roles role
		  ON role.username=membership.username AND role.group_id=membership.group_id
		ORDER BY membership.group_id,membership.username,role.role_id
	`)
	if err != nil {
		return nil, fmt.Errorf("list group memberships: %w", err)
	}
	defer rows.Close()
	result := make([]GroupMembership, 0)
	var current *GroupMembership
	for rows.Next() {
		var username, groupID string
		var roleID sql.NullString
		if err := rows.Scan(&username, &groupID, &roleID); err != nil {
			return nil, err
		}
		if current == nil || current.Username != username || current.GroupID != groupID {
			result = append(result, GroupMembership{Username: username, GroupID: groupID})
			current = &result[len(result)-1]
		}
		if roleID.Valid {
			current.RoleIDs = append(current.RoleIDs, roleID.String)
		}
	}
	return result, rows.Err()
}
func (s *AuthStore) ReplaceMemberships(ctx context.Context, username string, memberships []GroupMembership) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_memberships WHERE username=? COLLATE NOCASE`, username); err != nil {
		return err
	}
	groupIDs := make([]string, 0, len(memberships))
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, membership := range memberships {
		if membership.GroupID == "" {
			continue
		}
		groupIDs = append(groupIDs, membership.GroupID)
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_memberships(username,group_id,created_at) VALUES(?,?,?)`,
			username, membership.GroupID, now); err != nil {
			return fmt.Errorf("create group membership: %w", err)
		}
		seen := map[string]bool{}
		for _, roleID := range membership.RoleIDs {
			if roleID == "" || seen[roleID] {
				continue
			}
			seen[roleID] = true
			if _, err := tx.ExecContext(ctx, `INSERT INTO group_membership_roles(username,group_id,role_id) VALUES(?,?,?)`,
				username, membership.GroupID, roleID); err != nil {
				return fmt.Errorf("assign group role %s: %w", roleID, err)
			}
		}
	}
	sort.Strings(groupIDs)
	if _, err := tx.ExecContext(ctx, `UPDATE users SET group_ids=?,updated_at=? WHERE username=? COLLATE NOCASE`,
		encodeStringList(groupIDs), now, username); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *AuthStore) ListRBACRoles(ctx context.Context) ([]RBACRole, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,scope,description FROM rbac_roles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []RBACRole
	for rows.Next() {
		var role RBACRole
		if err := rows.Scan(&role.ID, &role.Name, &role.Scope, &role.Description); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}
