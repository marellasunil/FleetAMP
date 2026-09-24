package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

// SectionPolicyStore persists section delegation in SQLite.
type SectionPolicyStore struct{ db *sql.DB }

func (s *SectionPolicyStore) List(ctx context.Context) ([]configs.SectionPolicy, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT section_key, operator_editable
		FROM configuration_section_policies
	`)
	if err != nil {
		return nil, fmt.Errorf("list configuration section policies: %w", err)
	}
	defer rows.Close()
	overrides := make(map[string]bool)
	for rows.Next() {
		var key string
		var editable int
		if err := rows.Scan(&key, &editable); err != nil {
			return nil, err
		}
		overrides[key] = editable != 0
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	policies := configs.DefaultSectionPolicies()
	for index := range policies {
		if editable, ok := overrides[policies[index].SectionKey]; ok {
			policies[index].OperatorEditable = editable
		}
	}
	return policies, nil
}

func (s *SectionPolicyStore) SetOperatorEditable(ctx context.Context, sectionKey string, editable bool) error {
	if _, ok := configs.SectionDefinitionByKey(sectionKey); !ok {
		return fmt.Errorf("unsupported configuration section %q", sectionKey)
	}
	value := 0
	if editable {
		value = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO configuration_section_policies (section_key, operator_editable, updated_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ','now'))
		ON CONFLICT(section_key) DO UPDATE SET
			operator_editable = excluded.operator_editable,
			updated_at = excluded.updated_at
	`, sectionKey, value)
	if err != nil {
		return fmt.Errorf("update configuration section policy: %w", err)
	}
	return nil
}
