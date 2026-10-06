package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/integrations"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type IntegrationConnectionStore struct{ db *sql.DB }

func (s *IntegrationConnectionStore) Create(ctx context.Context, v *integrations.Connection) error {
	groups, e := json.Marshal(v.GroupIDs)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, `INSERT INTO integration_connections(id,name,provider,base_url,organization,project,repository,branch,allowed_root,mode,credential_ref,group_ids,enabled,created_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, v.ID, v.Name, v.Provider, v.BaseURL, v.Organization, v.Project, v.Repository, v.Branch, v.AllowedRoot, v.Mode, v.CredentialRef, string(groups), v.Enabled, v.CreatedBy, formatTime(v.CreatedAt))
	if e != nil {
		return fmt.Errorf("create integration connection: %w", e)
	}
	return nil
}
func (s *IntegrationConnectionStore) Get(ctx context.Context, id string) (*integrations.Connection, error) {
	v, e := scanIntegrationConnection(s.db.QueryRowContext(ctx, integrationConnectionSelect+` WHERE id=?`, id))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, storage.ErrIntegrationConnectionNotFound
	}
	return v, e
}
func (s *IntegrationConnectionStore) List(ctx context.Context, n int) ([]*integrations.Connection, error) {
	if n <= 0 || n > 500 {
		n = 100
	}
	rows, e := s.db.QueryContext(ctx, integrationConnectionSelect+` ORDER BY created_at DESC LIMIT ?`, n)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	r := []*integrations.Connection{}
	for rows.Next() {
		v, e := scanIntegrationConnection(rows)
		if e != nil {
			return nil, e
		}
		r = append(r, v)
	}
	return r, rows.Err()
}

const integrationConnectionSelect = `SELECT id,name,provider,base_url,organization,project,repository,branch,allowed_root,mode,credential_ref,group_ids,enabled,created_by,created_at FROM integration_connections`

type integrationConnectionScanner interface{ Scan(...any) error }

func scanIntegrationConnection(s integrationConnectionScanner) (*integrations.Connection, error) {
	v := &integrations.Connection{}
	var provider, groups, created string
	if e := s.Scan(&v.ID, &v.Name, &provider, &v.BaseURL, &v.Organization, &v.Project, &v.Repository, &v.Branch, &v.AllowedRoot, &v.Mode, &v.CredentialRef, &groups, &v.Enabled, &v.CreatedBy, &created); e != nil {
		return nil, e
	}
	v.Provider = integrations.ProviderID(provider)
	if e := json.Unmarshal([]byte(groups), &v.GroupIDs); e != nil {
		return nil, e
	}
	v.CreatedAt, _ = parseTime(created)
	return v, nil
}

var _ storage.IntegrationConnectionStore = (*IntegrationConnectionStore)(nil)
