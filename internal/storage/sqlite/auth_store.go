// SQLite persistence for the bootstrap administrator credential.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrAdministratorNotFound = errors.New("administrator not found")

type Administrator struct {
	Username          string
	PasswordSalt      []byte
	PasswordHash      []byte
	CreatedAt         time.Time
	PasswordChangedAt time.Time
}

type AuthStore struct{ db *sql.DB }

// Exists reports whether first-login setup has already created the singleton administrator.
func (s *AuthStore) Exists(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM administrators`).Scan(&count); err != nil {
		return false, fmt.Errorf("count administrators: %w", err)
	}
	return count > 0, nil
}

// Get loads the administrator name, salt, verifier, and derivation parameters.
func (s *AuthStore) Get(ctx context.Context) (*Administrator, error) {
	var admin Administrator
	var createdAt, changedAt string
	err := s.db.QueryRowContext(ctx, `
        SELECT username, password_salt, password_hash, created_at, password_changed_at
        FROM administrators WHERE singleton = 1
    `).Scan(&admin.Username, &admin.PasswordSalt, &admin.PasswordHash, &createdAt, &changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAdministratorNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get administrator: %w", err)
	}
	admin.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse administrator creation time: %w", err)
	}
	admin.PasswordChangedAt, err = time.Parse(time.RFC3339Nano, changedAt)
	if err != nil {
		return nil, fmt.Errorf("parse password change time: %w", err)
	}
	return &admin, nil
}

// Create inserts the first administrator and fails when setup was already completed.
func (s *AuthStore) Create(ctx context.Context, admin Administrator) error {
	now := time.Now().UTC()
	if admin.CreatedAt.IsZero() {
		admin.CreatedAt = now
	}
	if admin.PasswordChangedAt.IsZero() {
		admin.PasswordChangedAt = now
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO administrators (
            singleton, username, password_salt, password_hash, created_at, password_changed_at
        ) VALUES (1, ?, ?, ?, ?, ?)
    `, admin.Username, admin.PasswordSalt, admin.PasswordHash,
		admin.CreatedAt.Format(time.RFC3339Nano), admin.PasswordChangedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create administrator: %w", err)
	}
	return nil
}

// ReplacePassword atomically replaces verifier material for the named administrator.
func (s *AuthStore) ReplacePassword(ctx context.Context, username string, salt, hash []byte) error {
	result, err := s.db.ExecContext(ctx, `
        UPDATE administrators SET password_salt = ?, password_hash = ?, password_changed_at = ?
        WHERE singleton = 1 AND username = ?
    `, salt, hash, time.Now().UTC().Format(time.RFC3339Nano), username)
	if err != nil {
		return fmt.Errorf("replace administrator password: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect administrator update: %w", err)
	}
	if rows == 0 {
		return ErrAdministratorNotFound
	}
	return nil
}
