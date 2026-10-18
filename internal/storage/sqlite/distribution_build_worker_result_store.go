package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildWorkerResultStore struct{ db *sql.DB }

func (s *DistributionBuildWorkerResultStore) Create(ctx context.Context, result *distributions.BuildWorkerResult) error {
	if result == nil {
		return fmt.Errorf("build worker result is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_worker_results(id,execution_id,preflight_id,build_plan_hash,manifest_hash,worker_id,adapter,status,evidence_hash,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, result.ID, result.ExecutionID, result.PreflightID, result.BuildPlanHash, result.ManifestHash, result.WorkerID, result.Adapter, result.Status, result.EvidenceHash, result.CompletedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build worker result: %w", err)
	}
	return nil
}
func (s *DistributionBuildWorkerResultStore) List(ctx context.Context, limit int) ([]*distributions.BuildWorkerResult, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,execution_id,preflight_id,build_plan_hash,manifest_hash,worker_id,adapter,status,evidence_hash,completed_at FROM distribution_build_worker_results ORDER BY completed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildWorkerResult
	for rows.Next() {
		result := &distributions.BuildWorkerResult{}
		var completedAt string
		if err := rows.Scan(&result.ID, &result.ExecutionID, &result.PreflightID, &result.BuildPlanHash, &result.ManifestHash, &result.WorkerID, &result.Adapter, &result.Status, &result.EvidenceHash, &completedAt); err != nil {
			return nil, err
		}
		result.CompletedAt, _ = time.Parse(time.RFC3339Nano, completedAt)
		out = append(out, result)
	}
	return out, rows.Err()
}
