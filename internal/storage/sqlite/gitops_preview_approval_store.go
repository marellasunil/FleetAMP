package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GitOpsPreviewApprovalStore struct{ db *sql.DB }

func (s *GitOpsPreviewApprovalStore) Create(ctx context.Context, value *lifecycle.GitOpsPreviewApproval) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO gitops_preview_approvals(id,preview_id,preview_hash,plan_hash,connection_id,repository_path,branch,submitted_by,assigned_reviewer,submission_comment,status,reviewed_by,review_comment,reviewed_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.PreviewID, value.PreviewHash, value.PlanHash, value.ConnectionID, value.RepositoryPath, value.Branch, value.SubmittedBy, value.AssignedReviewer, value.SubmissionComment, value.Status, value.ReviewedBy, value.ReviewComment, formatTimePtr(value.ReviewedAt), formatTime(value.CreatedAt))
	if err != nil {
		return fmt.Errorf("create GitOps preview approval: %w", err)
	}
	return nil
}

func (s *GitOpsPreviewApprovalStore) Get(ctx context.Context, id string) (*lifecycle.GitOpsPreviewApproval, error) {
	value, err := scanGitOpsPreviewApproval(s.db.QueryRowContext(ctx, gitOpsPreviewApprovalSelect+` WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrGitOpsPreviewApprovalNotFound
	}
	return value, err
}

func (s *GitOpsPreviewApprovalStore) List(ctx context.Context, limit int) ([]*lifecycle.GitOpsPreviewApproval, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, gitOpsPreviewApprovalSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*lifecycle.GitOpsPreviewApproval{}
	for rows.Next() {
		value, err := scanGitOpsPreviewApproval(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *GitOpsPreviewApprovalStore) Review(ctx context.Context, id string, from, to lifecycle.ApprovalStatus, reviewer, comment string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE gitops_preview_approvals SET status=?,reviewed_by=?,review_comment=?,reviewed_at=? WHERE id=? AND status=?`, to, reviewer, comment, formatTime(now), id, from)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return storage.ErrGitOpsPreviewApprovalConflict
	}
	return nil
}

const gitOpsPreviewApprovalSelect = `SELECT id,preview_id,preview_hash,plan_hash,connection_id,repository_path,branch,submitted_by,assigned_reviewer,submission_comment,status,reviewed_by,review_comment,reviewed_at,created_at FROM gitops_preview_approvals`

type gitOpsPreviewApprovalScanner interface{ Scan(...any) error }

func scanGitOpsPreviewApproval(scanner gitOpsPreviewApprovalScanner) (*lifecycle.GitOpsPreviewApproval, error) {
	value := &lifecycle.GitOpsPreviewApproval{}
	var status, createdAt string
	var reviewedAt sql.NullString
	if err := scanner.Scan(&value.ID, &value.PreviewID, &value.PreviewHash, &value.PlanHash, &value.ConnectionID, &value.RepositoryPath, &value.Branch, &value.SubmittedBy, &value.AssignedReviewer, &value.SubmissionComment, &status, &value.ReviewedBy, &value.ReviewComment, &reviewedAt, &createdAt); err != nil {
		return nil, err
	}
	value.Status = lifecycle.ApprovalStatus(status)
	value.CreatedAt, _ = parseTime(createdAt)
	if reviewedAt.Valid {
		parsed, _ := parseTime(reviewedAt.String)
		value.ReviewedAt = &parsed
	}
	return value, nil
}

var _ storage.GitOpsPreviewApprovalStore = (*GitOpsPreviewApprovalStore)(nil)
