// SQLite persistence for FleetAMP users and RBAC roles.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrLastAdmin    = errors.New("the last enabled administrator cannot be changed")
)

type User struct {
	Username          string
	Role              string
	Enabled           bool
	PasswordSalt      []byte
	PasswordHash      []byte
	CreatedAt         time.Time
	PasswordChangedAt time.Time
	UpdatedAt         time.Time
}

type AuthStore struct{ db *sql.DB }

// Exists reports whether first-login setup has created at least one user.
func (s *AuthStore) Exists(ctx context.Context) (bool, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count > 0, nil
}

// Get loads one user by case-insensitive username.
func (s *AuthStore) Get(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx, userSelect+` WHERE username = ? COLLATE NOCASE`, username)
	user, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// List returns all users ordered by normalized username.
func (s *AuthStore) List(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx, userSelect+` ORDER BY username COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	users := make([]*User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// Create inserts a user with an immutable username.
func (s *AuthStore) Create(ctx context.Context, user User) error {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.PasswordChangedAt.IsZero() {
		user.PasswordChangedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}
	enabled := 0
	if user.Enabled {
		enabled = 1
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO users (
            username, role, enabled, password_salt, password_hash,
            created_at, password_changed_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `, user.Username, user.Role, enabled, user.PasswordSalt, user.PasswordHash,
		user.CreatedAt.Format(time.RFC3339Nano),
		user.PasswordChangedAt.Format(time.RFC3339Nano),
		user.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// ReplacePassword atomically replaces password verifier material.
func (s *AuthStore) ReplacePassword(ctx context.Context, username string, salt, hash []byte) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
        UPDATE users SET password_salt = ?, password_hash = ?,
            password_changed_at = ?, updated_at = ?
        WHERE username = ? COLLATE NOCASE
    `, salt, hash, now, now, username)
	if err != nil {
		return fmt.Errorf("replace user password: %w", err)
	}
	return affectedUser(result)
}

// UpdateRole changes a user's role while preserving at least one enabled Admin.
func (s *AuthStore) UpdateRole(ctx context.Context, username, nextRole string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	currentRole, enabled, err := userRoleState(ctx, tx, username)
	if err != nil {
		return err
	}
	if currentRole == "admin" && enabled && nextRole != "admin" {
		if err := requireAnotherAdmin(ctx, tx); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `
        UPDATE users SET role = ?, updated_at = ?
        WHERE username = ? COLLATE NOCASE
    `, nextRole, time.Now().UTC().Format(time.RFC3339Nano), username)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	if err := affectedUser(result); err != nil {
		return err
	}
	return tx.Commit()
}

// SetEnabled enables or disables a user while preserving an active Admin.
func (s *AuthStore) SetEnabled(ctx context.Context, username string, next bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	currentRole, enabled, err := userRoleState(ctx, tx, username)
	if err != nil {
		return err
	}
	if currentRole == "admin" && enabled && !next {
		if err := requireAnotherAdmin(ctx, tx); err != nil {
			return err
		}
	}
	value := 0
	if next {
		value = 1
	}
	result, err := tx.ExecContext(ctx, `
        UPDATE users SET enabled = ?, updated_at = ?
        WHERE username = ? COLLATE NOCASE
    `, value, time.Now().UTC().Format(time.RFC3339Nano), username)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	if err := affectedUser(result); err != nil {
		return err
	}
	return tx.Commit()
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func userRoleState(ctx context.Context, query queryRower, username string) (string, bool, error) {
	var role string
	var enabled int
	err := query.QueryRowContext(ctx,
		`SELECT role, enabled FROM users WHERE username = ? COLLATE NOCASE`,
		username).Scan(&role, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, ErrUserNotFound
	}
	return role, enabled != 0, err
}

func requireAnotherAdmin(ctx context.Context, query queryRower) error {
	var count int
	if err := query.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE role = 'admin' AND enabled = 1`).Scan(&count); err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func affectedUser(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

const userSelect = `SELECT username, role, enabled, password_salt, password_hash,
    created_at, password_changed_at, updated_at FROM users`

type userScanner interface{ Scan(...any) error }

func scanUser(scanner userScanner) (*User, error) {
	var user User
	var enabled int
	var createdAt, changedAt, updatedAt string
	if err := scanner.Scan(&user.Username, &user.Role, &enabled,
		&user.PasswordSalt, &user.PasswordHash,
		&createdAt, &changedAt, &updatedAt); err != nil {
		return nil, err
	}
	var err error
	user.Enabled = enabled != 0
	if user.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt); err != nil {
		return nil, err
	}
	if user.PasswordChangedAt, err = time.Parse(time.RFC3339Nano, changedAt); err != nil {
		return nil, err
	}
	if user.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		return nil, err
	}
	return &user, nil
}
