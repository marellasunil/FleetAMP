package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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

func (s *GitOpsExecutionStore) Claim(ctx context.Context, worker string, now, until time.Time) (*lifecycle.GitOpsExecutionRequest, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, gitOpsExecutionSelect+` WHERE id IN (SELECT r.id FROM gitops_execution_requests r JOIN gitops_execution_events e ON e.id=(SELECT e2.id FROM gitops_execution_events e2 WHERE e2.execution_id=r.id ORDER BY e2.created_at DESC,e2.id DESC LIMIT 1) LEFT JOIN gitops_execution_leases l ON l.execution_id=r.id WHERE (e.status=? AND (l.execution_id IS NULL OR l.lease_until<=?)) OR (e.status=? AND l.lease_until<=?) ORDER BY r.created_at LIMIT 20)`, lifecycle.GitOpsExecutionQueued, formatTime(now), lifecycle.GitOpsExecutionClaimed, formatTime(now))
	if err != nil {
		return nil, err
	}
	candidates := []*lifecycle.GitOpsExecutionRequest{}
	for rows.Next() {
		value, scanErr := scanGitOpsExecutionRequest(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		candidates = append(candidates, value)
	}
	rows.Close()
	for _, value := range candidates {
		result, execErr := tx.ExecContext(ctx, `INSERT INTO gitops_execution_leases(execution_id,worker_id,lease_until,claimed_at) VALUES(?,?,?,?) ON CONFLICT(execution_id) DO UPDATE SET worker_id=excluded.worker_id,lease_until=excluded.lease_until,claimed_at=excluded.claimed_at WHERE gitops_execution_leases.lease_until<=?`, value.ID, worker, formatTime(until), formatTime(now), formatTime(now))
		if execErr != nil {
			return nil, execErr
		}
		count, _ := result.RowsAffected()
		if count != 1 {
			continue
		}
		event, eventErr := lifecycle.NewGitOpsExecutionEvent(value.ID, lifecycle.GitOpsExecutionClaimed, worker, "Execution claimed by provider worker", map[string]string{"lease_until": until.UTC().Format(time.RFC3339Nano)})
		if eventErr != nil {
			return nil, eventErr
		}
		if eventErr = insertGitOpsExecutionEvent(ctx, tx, event); eventErr != nil {
			return nil, eventErr
		}
		if err = tx.Commit(); err != nil {
			return nil, err
		}
		return value, nil
	}
	return nil, storage.ErrGitOpsExecutionQueueEmpty
}

func (s *GitOpsExecutionStore) Complete(ctx context.Context, worker string, event *lifecycle.GitOpsExecutionEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner string
	if err = tx.QueryRowContext(ctx, `SELECT worker_id FROM gitops_execution_leases WHERE execution_id=?`, event.ExecutionID).Scan(&owner); errors.Is(err, sql.ErrNoRows) {
		return storage.ErrGitOpsExecutionLeaseConflict
	} else if err != nil {
		return err
	}
	if owner != worker {
		return storage.ErrGitOpsExecutionLeaseConflict
	}
	if err = insertGitOpsExecutionEvent(ctx, tx, event); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM gitops_execution_leases WHERE execution_id=? AND worker_id=?`, event.ExecutionID, worker); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *GitOpsExecutionStore) Retry(ctx context.Context, event *lifecycle.GitOpsExecutionEvent, maxAttempts int) error {
	if event == nil || event.Status != lifecycle.GitOpsExecutionQueued {
		return storage.ErrGitOpsExecutionNotRetryable
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var latest string
	if err = tx.QueryRowContext(ctx, `SELECT status FROM gitops_execution_events WHERE execution_id=? ORDER BY created_at DESC,id DESC LIMIT 1`, event.ExecutionID).Scan(&latest); errors.Is(err, sql.ErrNoRows) {
		return storage.ErrGitOpsExecutionNotFound
	} else if err != nil {
		return err
	}
	if lifecycle.GitOpsExecutionStatus(latest) != lifecycle.GitOpsExecutionFailed {
		return storage.ErrGitOpsExecutionNotRetryable
	}
	var attempts int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM gitops_execution_events WHERE execution_id=? AND status=?`, event.ExecutionID, lifecycle.GitOpsExecutionQueued).Scan(&attempts); err != nil {
		return err
	}
	if maxAttempts > 0 && attempts >= maxAttempts {
		return storage.ErrGitOpsExecutionNotRetryable
	}
	if err = insertGitOpsExecutionEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
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
