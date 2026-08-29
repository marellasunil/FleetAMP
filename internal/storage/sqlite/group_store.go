// SQLite-backed implementation of the FleetAMP GroupStore.
//
// Purpose:
//
//	Persists controlled group selectors and enabled state, including safe
//	serialization of Application/Environment/Place identity into SQLite.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GroupStore struct{ db *sql.DB }

func (s *GroupStore) Create(ctx context.Context, group *groups.Group) error {
	selector, err := json.Marshal(group.Selector)
	if err != nil {
		return fmt.Errorf("encode group selector: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO groups(id,name,description,selector,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
		group.ID, group.Name, group.Description, string(selector), boolToInt(group.Enabled), group.CreatedAt.Format(time.RFC3339Nano), group.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create group: %w", err)
	}
	return nil
}

func (s *GroupStore) Update(ctx context.Context, group *groups.Group) error {
	selector, err := json.Marshal(group.Selector)
	if err != nil {
		return fmt.Errorf("encode group selector: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE groups SET name=?,description=?,selector=?,enabled=?,updated_at=? WHERE id=?`,
		group.Name, group.Description, string(selector), boolToInt(group.Enabled), group.UpdatedAt.Format(time.RFC3339Nano), group.ID)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrGroupNotFound
	}
	return nil
}

func (s *GroupStore) Get(ctx context.Context, id string) (*groups.Group, error) {
	row := s.db.QueryRowContext(ctx, groupSelect+` WHERE id=?`, id)
	group, err := scanGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrGroupNotFound
	}
	return group, err
}

func (s *GroupStore) List(ctx context.Context) ([]*groups.Group, error) {
	rows, err := s.db.QueryContext(ctx, groupSelect+` ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()
	result := make([]*groups.Group, 0)
	for rows.Next() {
		group, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, group)
	}
	return result, rows.Err()
}

func (s *GroupStore) Delete(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM groups WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return storage.ErrGroupNotFound
	}
	return nil
}

const groupSelect = `SELECT id,name,description,selector,enabled,created_at,updated_at FROM groups`

type scanner interface{ Scan(dest ...any) error }

func scanGroup(s scanner) (*groups.Group, error) {
	var g groups.Group
	var selector, createdAt, updatedAt string
	var enabled int
	if err := s.Scan(&g.ID, &g.Name, &g.Description, &selector, &enabled, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	g.Enabled = enabled != 0
	if err := json.Unmarshal([]byte(selector), &g.Selector); err != nil {
		return nil, fmt.Errorf("decode group selector: %w", err)
	}
	var err error
	g.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, err
	}
	g.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

var _ storage.GroupStore = (*GroupStore)(nil)
