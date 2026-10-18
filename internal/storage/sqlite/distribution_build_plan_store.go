package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildPlanStore struct{ db *sql.DB }

func (s *DistributionBuildPlanStore) Create(ctx context.Context, p *distributions.BuildPlan) error {
	if p == nil {
		return fmt.Errorf("build plan is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_plans(id,distribution_id,manifest,plan_hash,status,requested_by,requested_at) VALUES(?,?,?,?,?,?,?)`, p.ID, p.DistributionID, p.ManifestJSON, p.PlanHash, p.Status, p.RequestedBy, p.RequestedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build plan: %w", err)
	}
	return nil
}
func (s *DistributionBuildPlanStore) List(ctx context.Context, limit int) ([]*distributions.BuildPlan, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,distribution_id,manifest,plan_hash,status,requested_by,requested_at FROM distribution_build_plans ORDER BY requested_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildPlan
	for rows.Next() {
		p := &distributions.BuildPlan{}
		var requested string
		if err := rows.Scan(&p.ID, &p.DistributionID, &p.ManifestJSON, &p.PlanHash, &p.Status, &p.RequestedBy, &requested); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(p.ManifestJSON), &p.Manifest); err != nil {
			return nil, err
		}
		p.RequestedAt, _ = time.Parse(time.RFC3339Nano, requested)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *DistributionBuildPlanStore) Get(ctx context.Context, id string) (*distributions.BuildPlan, error) {
	p := &distributions.BuildPlan{}
	var requested string
	err := s.db.QueryRowContext(ctx, `SELECT id,distribution_id,manifest,plan_hash,status,requested_by,requested_at FROM distribution_build_plans WHERE id=?`, id).Scan(&p.ID,&p.DistributionID,&p.ManifestJSON,&p.PlanHash,&p.Status,&p.RequestedBy,&requested)
	if err == sql.ErrNoRows { return nil, fmt.Errorf("build plan %q not found", id) }
	if err != nil { return nil, err }
	if err := json.Unmarshal([]byte(p.ManifestJSON), &p.Manifest); err != nil { return nil, err }
	p.RequestedAt, _ = time.Parse(time.RFC3339Nano, requested)
	return p, nil
}
