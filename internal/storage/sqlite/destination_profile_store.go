package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/blueprints"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type DestinationProfileStore struct{ db *sql.DB }

func (s *DestinationProfileStore) Create(ctx context.Context, p *blueprints.DestinationProfile) error {
	if p == nil {
		return fmt.Errorf("destination profile is required")
	}
	groupIDs, err := json.Marshal(p.GroupIDs)
	if err != nil {
		return fmt.Errorf("encode destination groups: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO destination_profiles(id,name,environment,exporter_id,exporter_config,owner,visibility,group_ids,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Environment, p.ExporterID, p.ExporterConfig, p.Owner, p.Visibility, string(groupIDs), boolToInt(p.Enabled), p.CreatedAt.Format(time.RFC3339Nano), p.UpdatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create destination profile: %w", err)
	}
	return nil
}
func (s *DestinationProfileStore) Get(ctx context.Context, id string) (*blueprints.DestinationProfile, error) {
	return scanDestination(s.db.QueryRowContext(ctx, destinationSelect+` WHERE id=?`, id))
}
func (s *DestinationProfileStore) List(ctx context.Context) ([]*blueprints.DestinationProfile, error) {
	rows, err := s.db.QueryContext(ctx, destinationSelect+` ORDER BY name,environment`)
	if err != nil {
		return nil, fmt.Errorf("list destination profiles: %w", err)
	}
	defer rows.Close()
	var out []*blueprints.DestinationProfile
	for rows.Next() {
		p, e := scanDestination(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *DestinationProfileStore) Delete(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, `DELETE FROM destination_profiles WHERE id=?`, id)
	if e != nil {
		return fmt.Errorf("delete destination profile: %w", e)
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return storage.ErrDestinationProfileNotFound
	}
	return nil
}

const destinationSelect = `SELECT id,name,environment,exporter_id,exporter_config,owner,visibility,group_ids,enabled,created_at,updated_at FROM destination_profiles`

func scanDestination(s scanner) (*blueprints.DestinationProfile, error) {
	var p blueprints.DestinationProfile
	var enabled int
	var created, updated, groupIDs string
	if e := s.Scan(&p.ID, &p.Name, &p.Environment, &p.ExporterID, &p.ExporterConfig, &p.Owner, &p.Visibility, &groupIDs, &enabled, &created, &updated); e != nil {
		if errors.Is(e, sql.ErrNoRows) {
			return nil, storage.ErrDestinationProfileNotFound
		}
		return nil, e
	}
	p.Enabled = enabled != 0
	if e := json.Unmarshal([]byte(groupIDs), &p.GroupIDs); e != nil {
		return nil, fmt.Errorf("decode destination groups: %w", e)
	}
	var e error
	p.CreatedAt, e = time.Parse(time.RFC3339Nano, created)
	if e != nil {
		return nil, e
	}
	p.UpdatedAt, e = time.Parse(time.RFC3339Nano, updated)
	return &p, e
}

var _ storage.DestinationProfileStore = (*DestinationProfileStore)(nil)
