package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GitOpsExecutionStore struct{ db *sql.DB }

func (s *GitOpsExecutionStore) Create(ctx context.Context, request *lifecycle.GitOpsExecutionRequest, event *lifecycle.GitOpsExecutionEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO gitops_execution_requests(id,approval_id,preview_id,preview_hash,plan_hash,connection_id,provider,mode,repository_path,branch,requested_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, request.ID, request.ApprovalID, request.PreviewID, request.PreviewHash, request.PlanHash, request.ConnectionID, request.Provider, request.Mode, request.RepositoryPath, request.Branch, request.RequestedBy, formatTime(request.CreatedAt))
	if err != nil {
		return fmt.Errorf("create GitOps execution request: %w", err)
	}
	if err = insertGitOpsExecutionEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *GitOpsExecutionStore) Get(ctx context.Context, id string) (*lifecycle.GitOpsExecutionRequest, error) {
	v, err := scanGitOpsExecutionRequest(s.db.QueryRowContext(ctx, gitOpsExecutionSelect+` WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrGitOpsExecutionNotFound
	}
	return v, err
}
func (s *GitOpsExecutionStore) List(ctx context.Context, limit int) ([]*lifecycle.GitOpsExecutionRequest, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, gitOpsExecutionSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*lifecycle.GitOpsExecutionRequest{}
	for rows.Next() {
		v, err := scanGitOpsExecutionRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *GitOpsExecutionStore) AppendEvent(ctx context.Context, event *lifecycle.GitOpsExecutionEvent) error {
	return insertGitOpsExecutionEvent(ctx, s.db, event)
}
func (s *GitOpsExecutionStore) ListEvents(ctx context.Context, id string) ([]*lifecycle.GitOpsExecutionEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,execution_id,status,actor,message,evidence,created_at FROM gitops_execution_events WHERE execution_id=? ORDER BY created_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []*lifecycle.GitOpsExecutionEvent{}
	for rows.Next() {
		v := &lifecycle.GitOpsExecutionEvent{}
		var status, evidence, created string
		if err := rows.Scan(&v.ID, &v.ExecutionID, &status, &v.Actor, &v.Message, &evidence, &created); err != nil {
			return nil, err
		}
		v.Status = lifecycle.GitOpsExecutionStatus(status)
		_ = json.Unmarshal([]byte(evidence), &v.Evidence)
		v.CreatedAt, _ = parseTime(created)
		result = append(result, v)
	}
	return result, rows.Err()
}

const gitOpsExecutionSelect = `SELECT id,approval_id,preview_id,preview_hash,plan_hash,connection_id,provider,mode,repository_path,branch,requested_by,created_at FROM gitops_execution_requests`

type gitOpsExecutionScanner interface{ Scan(...any) error }

func scanGitOpsExecutionRequest(scanner gitOpsExecutionScanner) (*lifecycle.GitOpsExecutionRequest, error) {
	v := &lifecycle.GitOpsExecutionRequest{}
	var created string
	if err := scanner.Scan(&v.ID, &v.ApprovalID, &v.PreviewID, &v.PreviewHash, &v.PlanHash, &v.ConnectionID, &v.Provider, &v.Mode, &v.RepositoryPath, &v.Branch, &v.RequestedBy, &created); err != nil {
		return nil, err
	}
	v.CreatedAt, _ = parseTime(created)
	return v, nil
}

type gitOpsExecutionExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertGitOpsExecutionEvent(ctx context.Context, execer gitOpsExecutionExecer, event *lifecycle.GitOpsExecutionEvent) error {
	evidence, _ := json.Marshal(event.Evidence)
	_, err := execer.ExecContext(ctx, `INSERT INTO gitops_execution_events(id,execution_id,status,actor,message,evidence,created_at) VALUES(?,?,?,?,?,?,?)`, event.ID, event.ExecutionID, event.Status, event.Actor, event.Message, string(evidence), formatTime(event.CreatedAt))
	return err
}

var _ storage.GitOpsExecutionStore = (*GitOpsExecutionStore)(nil)
