package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildSourceStore struct{ db *sql.DB }

func (s *DistributionBuildSourceStore) Create(ctx context.Context, item *distributions.BuildSourceResolution) error {
	if item == nil {
		return fmt.Errorf("build source resolution is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_sources(id,execution_id,worker_result_id,build_plan_hash,repository,source_ref,archive_url,status,evidence_hash,resolved_by,resolved_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.ExecutionID, item.WorkerResultID, item.BuildPlanHash, item.Repository, item.SourceRef, item.ArchiveURL, item.Status, item.EvidenceHash, item.ResolvedBy, item.ResolvedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build source resolution: %w", err)
	}
	return nil
}
func (s *DistributionBuildSourceStore) List(ctx context.Context, limit int) ([]*distributions.BuildSourceResolution, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,execution_id,worker_result_id,build_plan_hash,repository,source_ref,archive_url,status,evidence_hash,resolved_by,resolved_at FROM distribution_build_sources ORDER BY resolved_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildSourceResolution
	for rows.Next() {
		item := &distributions.BuildSourceResolution{}
		var resolvedAt string
		if err := rows.Scan(&item.ID, &item.ExecutionID, &item.WorkerResultID, &item.BuildPlanHash, &item.Repository, &item.SourceRef, &item.ArchiveURL, &item.Status, &item.EvidenceHash, &item.ResolvedBy, &resolvedAt); err != nil {
			return nil, err
		}
		item.ResolvedAt, _ = time.Parse(time.RFC3339Nano, resolvedAt)
		out = append(out, item)
	}
	return out, rows.Err()
}
