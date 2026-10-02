package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	groupsecrets "github.com/marellasunil/FleetAMP/internal/secrets"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GroupSecretStore struct{ db *sql.DB }

func (s *GroupSecretStore) Upsert(ctx context.Context, item *groupsecrets.GroupSecret) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO group_secrets(group_id,secret_key,ciphertext,updated_by,created_at,updated_at)
		VALUES(?,?,?,?,?,?) ON CONFLICT(group_id,secret_key) DO UPDATE SET ciphertext=excluded.ciphertext,updated_by=excluded.updated_by,updated_at=excluded.updated_at`,
		item.GroupID, item.Key, item.Ciphertext, item.UpdatedBy, item.CreatedAt.Format(time.RFC3339Nano), item.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *GroupSecretStore) Get(ctx context.Context, groupID, key string) (*groupsecrets.GroupSecret, error) {
	return scanGroupSecret(s.db.QueryRowContext(ctx, `SELECT group_id,secret_key,ciphertext,updated_by,created_at,updated_at FROM group_secrets WHERE group_id=? AND secret_key=?`, groupID, key))
}

func (s *GroupSecretStore) ListByGroup(ctx context.Context, groupID string) ([]*groupsecrets.GroupSecret, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_id,secret_key,ciphertext,updated_by,created_at,updated_at FROM group_secrets WHERE group_id=? ORDER BY secret_key`, groupID)
	if err != nil { return nil, err }
	defer rows.Close()
	var result []*groupsecrets.GroupSecret
	for rows.Next() { item, scanErr := scanGroupSecret(rows); if scanErr != nil { return nil, scanErr }; result = append(result, item) }
	return result, rows.Err()
}

func (s *GroupSecretStore) Delete(ctx context.Context, groupID, key string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM group_secrets WHERE group_id=? AND secret_key=?`, groupID, key)
	if err != nil { return err }
	if count, _ := result.RowsAffected(); count == 0 { return storage.ErrGroupSecretNotFound }
	return nil
}

func scanGroupSecret(row scanner) (*groupsecrets.GroupSecret, error) {
	var item groupsecrets.GroupSecret
	var created, updated string
	if err := row.Scan(&item.GroupID, &item.Key, &item.Ciphertext, &item.UpdatedBy, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) { return nil, storage.ErrGroupSecretNotFound }
		return nil, err
	}
	item.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	item.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return &item, nil
}

var _ storage.GroupSecretStore = (*GroupSecretStore)(nil)
