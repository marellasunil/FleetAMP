package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildAttemptStore struct{ db *sql.DB }

func (s *DistributionBuildAttemptStore) Create(ctx context.Context, item *distributions.BuildAttempt) error {
	if item == nil {
		return fmt.Errorf("build attempt is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_attempts(id,inspection_id,execution_id,build_plan_hash,manifest_sha256,attempt_number,trigger,retry_request_id,status,failure_code,failure_message,failure_digest,artifact_id,artifact_sha256,worker_id,evidence_hash,started_at,completed_at,next_retry_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.InspectionID, item.ExecutionID, item.BuildPlanHash, item.ManifestSHA256, item.AttemptNumber, item.Trigger, item.RetryRequestID, item.Status, item.FailureCode, item.FailureMessage, item.FailureDigest, item.ArtifactID, item.ArtifactSHA256, item.WorkerID, item.EvidenceHash, item.StartedAt.Format(time.RFC3339Nano), item.CompletedAt.Format(time.RFC3339Nano), formatOptionalTime(item.NextRetryAt))
	if err != nil {
		return fmt.Errorf("create distribution build attempt: %w", err)
	}
	return nil
}
func (s *DistributionBuildAttemptStore) List(ctx context.Context, limit int) ([]*distributions.BuildAttempt, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,inspection_id,execution_id,build_plan_hash,manifest_sha256,attempt_number,trigger,retry_request_id,status,failure_code,failure_message,failure_digest,artifact_id,artifact_sha256,worker_id,evidence_hash,started_at,completed_at,next_retry_at FROM distribution_build_attempts ORDER BY completed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildAttempt
	for rows.Next() {
		item := &distributions.BuildAttempt{}
		var started, completed, next string
		if err := rows.Scan(&item.ID, &item.InspectionID, &item.ExecutionID, &item.BuildPlanHash, &item.ManifestSHA256, &item.AttemptNumber, &item.Trigger, &item.RetryRequestID, &item.Status, &item.FailureCode, &item.FailureMessage, &item.FailureDigest, &item.ArtifactID, &item.ArtifactSHA256, &item.WorkerID, &item.EvidenceHash, &started, &completed, &next); err != nil {
			return nil, err
		}
		item.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		item.CompletedAt, _ = time.Parse(time.RFC3339Nano, completed)
		if next != "" {
			item.NextRetryAt, _ = time.Parse(time.RFC3339Nano, next)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

type DistributionBuildRetryRequestStore struct{ db *sql.DB }

func (s *DistributionBuildRetryRequestStore) Create(ctx context.Context, item *distributions.BuildRetryRequest) error {
	if item == nil {
		return fmt.Errorf("build retry request is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_retry_requests(id,inspection_id,previous_attempt_id,reason,requested_by,evidence_hash,requested_at) VALUES(?,?,?,?,?,?,?)`, item.ID, item.InspectionID, item.PreviousAttemptID, item.Reason, item.RequestedBy, item.EvidenceHash, item.RequestedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build retry request: %w", err)
	}
	return nil
}
func (s *DistributionBuildRetryRequestStore) List(ctx context.Context, limit int) ([]*distributions.BuildRetryRequest, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,inspection_id,previous_attempt_id,reason,requested_by,evidence_hash,requested_at FROM distribution_build_retry_requests ORDER BY requested_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildRetryRequest
	for rows.Next() {
		item := &distributions.BuildRetryRequest{}
		var at string
		if err := rows.Scan(&item.ID, &item.InspectionID, &item.PreviousAttemptID, &item.Reason, &item.RequestedBy, &item.EvidenceHash, &at); err != nil {
			return nil, err
		}
		item.RequestedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}

func formatOptionalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
