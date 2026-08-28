// SQLite-backed implementation of FleetAMP ConfigurationStore.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ConfigStore struct{ db *sql.DB }

func (s *ConfigStore) Put(ctx context.Context, config *configs.Configuration) error {
	if config == nil {
		return fmt.Errorf("configuration is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO configurations
        (id,name,version,content,content_type,hash,created_at) VALUES(?,?,?,?,?,?,?)
        ON CONFLICT(id) DO NOTHING`, config.ID, config.Name, config.Version, config.Content,
		config.ContentType, config.Hash, config.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("store configuration: %w", err)
	}
	return nil
}

func (s *ConfigStore) Get(ctx context.Context, id string) (*configs.Configuration, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,version,content,content_type,hash,created_at FROM configurations WHERE id=?`, id)
	return scanConfiguration(row)
}

func (s *ConfigStore) List(ctx context.Context) ([]*configs.Configuration, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,version,content,content_type,hash,created_at FROM configurations ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, fmt.Errorf("list configurations: %w", err)
	}
	defer rows.Close()
	result := make([]*configs.Configuration, 0)
	for rows.Next() {
		config, err := scanConfiguration(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, config)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list configurations: %w", err)
	}
	return result, nil
}

type configScanner interface{ Scan(dest ...any) error }

func scanConfiguration(scanner configScanner) (*configs.Configuration, error) {
	var config configs.Configuration
	var created string
	if err := scanner.Scan(&config.ID, &config.Name, &config.Version, &config.Content, &config.ContentType, &config.Hash, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrConfigurationNotFound
		}
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return nil, fmt.Errorf("parse configuration created_at: %w", err)
	}
	config.CreatedAt = parsed
	return &config, nil
}

var _ storage.ConfigurationStore = (*ConfigStore)(nil)
