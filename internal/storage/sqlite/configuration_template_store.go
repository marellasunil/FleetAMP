package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ConfigurationTemplateStore struct{ db *sql.DB }

func (s *ConfigurationTemplateStore) Create(ctx context.Context, item *configs.Template) error {
	if item == nil {
		return fmt.Errorf("configuration template is required")
	}
	groupIDs, err := json.Marshal(item.GroupIDs)
	if err != nil {
		return fmt.Errorf("encode configuration template groups: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO configuration_templates
		(id,name,version,description,content,content_hash,visibility,group_ids,source_configuration_id,created_by,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		item.ID, item.Name, item.Version, item.Description, item.Content, item.ContentHash, string(item.Visibility),
		string(groupIDs), item.SourceConfigurationID, item.CreatedBy, item.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create configuration template: %w", err)
	}
	return nil
}

func (s *ConfigurationTemplateStore) Get(ctx context.Context, id string) (*configs.Template, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,version,description,content,content_hash,visibility,group_ids,
		source_configuration_id,created_by,created_at FROM configuration_templates WHERE id=?`, id)
	return scanConfigurationTemplate(row)
}

func (s *ConfigurationTemplateStore) List(ctx context.Context) ([]*configs.Template, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,version,description,content,content_hash,visibility,group_ids,
		source_configuration_id,created_by,created_at FROM configuration_templates ORDER BY created_at DESC,id`)
	if err != nil {
		return nil, fmt.Errorf("list configuration templates: %w", err)
	}
	defer rows.Close()
	result := make([]*configs.Template, 0)
	for rows.Next() {
		item, err := scanConfigurationTemplate(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list configuration templates: %w", err)
	}
	return result, nil
}

type configurationTemplateScanner interface{ Scan(...any) error }

func scanConfigurationTemplate(scanner configurationTemplateScanner) (*configs.Template, error) {
	var item configs.Template
	var visibility, groupIDs, createdAt string
	if err := scanner.Scan(&item.ID, &item.Name, &item.Version, &item.Description, &item.Content, &item.ContentHash,
		&visibility, &groupIDs, &item.SourceConfigurationID, &item.CreatedBy, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrConfigurationTemplateNotFound
		}
		return nil, fmt.Errorf("read configuration template: %w", err)
	}
	item.Visibility = configs.TemplateVisibility(visibility)
	if err := json.Unmarshal([]byte(groupIDs), &item.GroupIDs); err != nil {
		return nil, fmt.Errorf("decode configuration template groups: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse configuration template created_at: %w", err)
	}
	item.CreatedAt = parsed
	return &item, nil
}

var _ storage.ConfigurationTemplateStore = (*ConfigurationTemplateStore)(nil)
