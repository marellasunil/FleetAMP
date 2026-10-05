package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/migrations"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

// MigrationStore persists migration provenance beside immutable configurations.
type MigrationStore struct{ db *sql.DB }

func (s *MigrationStore) Save(ctx context.Context, configuration *configs.Configuration, record *migrations.Record) error {
	if configuration == nil || record == nil {
		return fmt.Errorf("configuration and migration record are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration save: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO configurations
		(id,group_id,name,version,content,content_type,hash,created_at) VALUES(?,?,?,?,?,?,?,?)`,
		configuration.ID, configuration.GroupID, configuration.Name, configuration.Version, configuration.Content,
		configuration.ContentType, configuration.Hash, configuration.CreatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("store migrated configuration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO migration_history
		(id,configuration_id,group_id,group_name,name,version,source,agent_uid,pattern_id,content_hash,created_by,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, record.ID, record.ConfigurationID, record.GroupID, record.GroupName,
		record.Name, record.Version, record.Source, record.AgentUID, record.PatternID, record.ContentHash,
		record.CreatedBy, record.CreatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("store migration history: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration save: %w", err)
	}
	return nil
}

func (s *MigrationStore) List(ctx context.Context) ([]*migrations.Record, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,configuration_id,group_id,group_name,name,version,source,agent_uid,pattern_id,content_hash,created_by,created_at
		FROM migration_history ORDER BY created_at DESC,id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list migration history: %w", err)
	}
	defer rows.Close()
	result := []*migrations.Record{}
	for rows.Next() {
		record := &migrations.Record{}
		var created string
		if err := rows.Scan(&record.ID, &record.ConfigurationID, &record.GroupID, &record.GroupName, &record.Name, &record.Version,
			&record.Source, &record.AgentUID, &record.PatternID, &record.ContentHash, &record.CreatedBy, &created); err != nil {
			return nil, fmt.Errorf("read migration history: %w", err)
		}
		record.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse migration history timestamp: %w", err)
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list migration history: %w", err)
	}
	return result, nil
}

var _ storage.MigrationStore = (*MigrationStore)(nil)
