package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type IntegrationValidationStore struct{ db *sql.DB }

func (s *IntegrationValidationStore) Create(ctx context.Context, v *integrations.ConnectionValidation) error {
	evidence, err := json.Marshal(v.Evidence)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO integration_connection_validations(id,connection_id,provider,status,message,evidence,validated_by,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?,?)`, v.ID, v.ConnectionID, v.Provider, v.Status, v.Message, string(evidence), v.ValidatedBy, formatTime(v.CreatedAt), formatTime(v.ExpiresAt))
	if err != nil {
		return fmt.Errorf("create integration validation: %w", err)
	}
	return nil
}

func (s *IntegrationValidationStore) Latest(ctx context.Context, connectionID string) (*integrations.ConnectionValidation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,connection_id,provider,status,message,evidence,validated_by,created_at,expires_at FROM integration_connection_validations WHERE connection_id=? ORDER BY created_at DESC,id DESC LIMIT 1`, connectionID)
	v := &integrations.ConnectionValidation{}
	var provider, evidence, created, expires string
	if err := row.Scan(&v.ID, &v.ConnectionID, &provider, &v.Status, &v.Message, &evidence, &v.ValidatedBy, &created, &expires); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrIntegrationValidationNotFound
		}
		return nil, err
	}
	v.Provider = integrations.ProviderID(provider)
	if err := json.Unmarshal([]byte(evidence), &v.Evidence); err != nil {
		return nil, err
	}
	var err error
	if v.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if v.ExpiresAt, err = parseTime(expires); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *IntegrationValidationStore) LatestUsable(ctx context.Context, connectionID string, now time.Time) (*integrations.ConnectionValidation, error) {
	v, err := s.Latest(ctx, connectionID)
	if err != nil || !v.Usable(now) {
		return nil, storage.ErrIntegrationValidationNotFound
	}
	return v, nil
}
