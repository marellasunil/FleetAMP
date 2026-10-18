package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildPreflightStore struct{ db *sql.DB }

func (s *DistributionBuildPreflightStore) Create(ctx context.Context, receipt *distributions.BuildPreflight) error {
	if receipt == nil {
		return fmt.Errorf("build preflight is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_preflights(id,execution_id,build_plan_id,build_plan_hash,manifest_hash,status,checked_by,checked_at) VALUES(?,?,?,?,?,?,?,?)`, receipt.ID, receipt.ExecutionID, receipt.BuildPlanID, receipt.BuildPlanHash, receipt.ManifestHash, receipt.Status, receipt.CheckedBy, receipt.CheckedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build preflight: %w", err)
	}
	return nil
}
func (s *DistributionBuildPreflightStore) List(ctx context.Context, limit int) ([]*distributions.BuildPreflight, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,execution_id,build_plan_id,build_plan_hash,manifest_hash,status,checked_by,checked_at FROM distribution_build_preflights ORDER BY checked_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildPreflight
	for rows.Next() {
		receipt := &distributions.BuildPreflight{}
		var checkedAt string
		if err := rows.Scan(&receipt.ID, &receipt.ExecutionID, &receipt.BuildPlanID, &receipt.BuildPlanHash, &receipt.ManifestHash, &receipt.Status, &receipt.CheckedBy, &checkedAt); err != nil {
			return nil, err
		}
		receipt.CheckedAt, _ = time.Parse(time.RFC3339Nano, checkedAt)
		out = append(out, receipt)
	}
	return out, rows.Err()
}
