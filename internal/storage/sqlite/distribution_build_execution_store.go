package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionBuildExecutionStore struct{ db *sql.DB }

func (s *DistributionBuildExecutionStore) Create(ctx context.Context, execution *distributions.BuildExecution) error {
	if execution == nil {
		return fmt.Errorf("build execution is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_executions(id,build_plan_id,build_plan_hash,approval_id,approval_reviewed_by,approval_reviewed_at,manifest,executor,status,queued_by,queued_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, execution.ID, execution.BuildPlanID, execution.BuildPlanHash, execution.ApprovalID, execution.ApprovalReviewedBy, execution.ApprovalReviewedAt.Format(time.RFC3339Nano), execution.ManifestJSON, execution.Executor, execution.Status, execution.QueuedBy, execution.QueuedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build execution: %w", err)
	}
	return nil
}

func (s *DistributionBuildExecutionStore) List(ctx context.Context, limit int) ([]*distributions.BuildExecution, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,build_plan_id,build_plan_hash,approval_id,approval_reviewed_by,approval_reviewed_at,manifest,executor,status,queued_by,queued_at FROM distribution_build_executions ORDER BY queued_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildExecution
	for rows.Next() {
		execution := &distributions.BuildExecution{}
		var reviewedAt, queuedAt string
		if err := rows.Scan(&execution.ID, &execution.BuildPlanID, &execution.BuildPlanHash, &execution.ApprovalID, &execution.ApprovalReviewedBy, &reviewedAt, &execution.ManifestJSON, &execution.Executor, &execution.Status, &execution.QueuedBy, &queuedAt); err != nil {
			return nil, err
		}
		execution.ApprovalReviewedAt, _ = time.Parse(time.RFC3339Nano, reviewedAt)
		execution.QueuedAt, _ = time.Parse(time.RFC3339Nano, queuedAt)
		out = append(out, execution)
	}
	return out, rows.Err()
}

func (s *DistributionBuildExecutionStore) Get(ctx context.Context, id string) (*distributions.BuildExecution, error) {
	execution := &distributions.BuildExecution{}
	var reviewedAt, queuedAt string
	err := s.db.QueryRowContext(ctx, `SELECT id,build_plan_id,build_plan_hash,approval_id,approval_reviewed_by,approval_reviewed_at,manifest,executor,status,queued_by,queued_at FROM distribution_build_executions WHERE id=?`, id).Scan(&execution.ID, &execution.BuildPlanID, &execution.BuildPlanHash, &execution.ApprovalID, &execution.ApprovalReviewedBy, &reviewedAt, &execution.ManifestJSON, &execution.Executor, &execution.Status, &execution.QueuedBy, &queuedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("build execution %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	execution.ApprovalReviewedAt, _ = time.Parse(time.RFC3339Nano, reviewedAt)
	execution.QueuedAt, _ = time.Parse(time.RFC3339Nano, queuedAt)
	return execution, nil
}
