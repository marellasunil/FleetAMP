package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
	"time"
)

type MembershipChangeStore struct{ db *sql.DB }

func (s *MembershipChangeStore) Create(ctx context.Context, v *configs.MembershipChange) error {
	targets, e := json.Marshal(v.Targets)
	if e != nil {
		return e
	}
	results, e := json.Marshal(v.ExecutionResults)
	if e != nil {
		return e
	}
	_, e = s.db.ExecContext(ctx, `INSERT INTO membership_changes(id,operation,source_group_id,source_group_name,target_group_id,target_group_name,removal_policy,target_configuration_id,targets,requested_by,assigned_reviewer,reason,status,reviewed_by,review_comment,reviewed_at,error,execution_results,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, v.ID, v.Operation, v.SourceGroupID, v.SourceGroupName, v.TargetGroupID, v.TargetGroupName, v.RemovalPolicy, v.TargetConfigurationID, string(targets), v.RequestedBy, v.AssignedReviewer, v.Reason, v.Status, v.ReviewedBy, v.ReviewComment, formatTimePtr(v.ReviewedAt), v.Error, string(results), formatTime(v.CreatedAt))
	if e != nil {
		return fmt.Errorf("create membership change: %w", e)
	}
	return nil
}
func (s *MembershipChangeStore) Get(ctx context.Context, id string) (*configs.MembershipChange, error) {
	v, e := scanMembership(s.db.QueryRowContext(ctx, membershipSelect+` WHERE id=?`, id))
	if e == sql.ErrNoRows {
		return nil, storage.ErrMembershipChangeNotFound
	}
	return v, e
}
func (s *MembershipChangeStore) List(ctx context.Context, limit int) ([]*configs.MembershipChange, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, e := s.db.QueryContext(ctx, membershipSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []*configs.MembershipChange
	for rows.Next() {
		v, e := scanMembership(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *MembershipChangeStore) SaveExecutionResults(ctx context.Context, id string, results []configs.MembershipExecutionResult) error {
	encoded, err := json.Marshal(results)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE membership_changes SET execution_results=? WHERE id=? AND status=?`, string(encoded), id, configs.MembershipExecuting)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return storage.ErrMembershipChangeConflict
	}
	return nil
}

func (s *MembershipChangeStore) Update(ctx context.Context, id string, from, to configs.MembershipStatus, reviewer, comment, errorText string) error {
	var reviewedAt any
	if reviewer != "" {
		reviewedAt = formatTime(time.Now().UTC())
	}
	result, e := s.db.ExecContext(ctx, `UPDATE membership_changes SET status=?,reviewed_by=?,review_comment=?,reviewed_at=COALESCE(?,reviewed_at),error=? WHERE id=? AND status=?`, to, reviewer, comment, reviewedAt, errorText, id, from)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return storage.ErrMembershipChangeConflict
	}
	return nil
}

const membershipSelect = `SELECT id,operation,source_group_id,source_group_name,target_group_id,target_group_name,removal_policy,target_configuration_id,targets,requested_by,assigned_reviewer,reason,status,reviewed_by,review_comment,reviewed_at,error,execution_results,created_at FROM membership_changes`

type membershipScanner interface{ Scan(...any) error }

func scanMembership(s membershipScanner) (*configs.MembershipChange, error) {
	v := &configs.MembershipChange{}
	var op, policy, status, targets, results, created string
	var reviewed sql.NullString
	if e := s.Scan(&v.ID, &op, &v.SourceGroupID, &v.SourceGroupName, &v.TargetGroupID, &v.TargetGroupName, &policy, &v.TargetConfigurationID, &targets, &v.RequestedBy, &v.AssignedReviewer, &v.Reason, &status, &v.ReviewedBy, &v.ReviewComment, &reviewed, &v.Error, &results, &created); e != nil {
		return nil, e
	}
	v.Operation = configs.MembershipOperation(op)
	v.RemovalPolicy = configs.RemovalPolicy(policy)
	v.Status = configs.MembershipStatus(status)
	if e := json.Unmarshal([]byte(targets), &v.Targets); e != nil {
		return nil, e
	}
	if results != "" && results != "null" {
		if e := json.Unmarshal([]byte(results), &v.ExecutionResults); e != nil {
			return nil, e
		}
	}
	v.CreatedAt, _ = parseTime(created)
	if reviewed.Valid {
		t, _ := parseTime(reviewed.String)
		v.ReviewedAt = &t
	}
	return v, nil
}

var _ storage.MembershipChangeStore = (*MembershipChangeStore)(nil)
