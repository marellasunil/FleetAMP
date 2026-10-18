package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildApprovalStore struct{ db *sql.DB }

func (s *DistributionBuildApprovalStore) Create(ctx context.Context, a *distributions.BuildApproval) error {
	if a == nil {
		return fmt.Errorf("build approval is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_approvals(id,build_plan_id,build_plan_hash,decision,reason,reviewed_by,reviewed_at) VALUES(?,?,?,?,?,?,?)`, a.ID, a.BuildPlanID, a.BuildPlanHash, a.Decision, a.Reason, a.ReviewedBy, a.ReviewedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build approval: %w", err)
	}
	return nil
}
func (s *DistributionBuildApprovalStore) List(ctx context.Context, limit int) ([]*distributions.BuildApproval, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,build_plan_id,build_plan_hash,decision,reason,reviewed_by,reviewed_at FROM distribution_build_approvals ORDER BY reviewed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildApproval
	for rows.Next() {
		a := &distributions.BuildApproval{}
		var reviewed string
		if err := rows.Scan(&a.ID, &a.BuildPlanID, &a.BuildPlanHash, &a.Decision, &a.Reason, &a.ReviewedBy, &reviewed); err != nil {
			return nil, err
		}
		a.ReviewedAt, _ = time.Parse(time.RFC3339Nano, reviewed)
		out = append(out, a)
	}
	return out, rows.Err()
}
