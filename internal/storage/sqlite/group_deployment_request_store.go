package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type GroupDeploymentRequestStore struct{ db *sql.DB }

func (s *GroupDeploymentRequestStore) Create(ctx context.Context, request *configs.GroupDeploymentRequest) error {
	selector, err := json.Marshal(request.GroupSelector)
	if err != nil {
		return fmt.Errorf("encode group selector: %w", err)
	}
	targets, err := json.Marshal(request.Targets)
	if err != nil {
		return fmt.Errorf("encode deployment targets: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO group_deployment_requests
        (id,group_id,group_name,group_selector,configuration_id,configuration_name,configuration_version,configuration_hash,base_configuration_id,base_configuration_hash,targets,requested_by,reviewed_by,review_comment,reviewed_at,status,created_at)
        VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, request.ID, request.GroupID, request.GroupName, string(selector),
		request.ConfigurationID, request.ConfigurationName, request.ConfigurationVersion, request.ConfigurationHash,
		request.BaseConfigurationID, request.BaseConfigurationHash, string(targets), request.RequestedBy,
		request.ReviewedBy, request.ReviewComment, formatTimePtr(request.ReviewedAt), string(request.Status), formatTime(request.CreatedAt))
	if err != nil {
		return fmt.Errorf("create group deployment request: %w", err)
	}
	return nil
}

func (s *GroupDeploymentRequestStore) Get(ctx context.Context, id string) (*configs.GroupDeploymentRequest, error) {
	row := s.db.QueryRowContext(ctx, groupDeploymentRequestSelect+` WHERE id=?`, id)
	item, err := scanGroupDeploymentRequest(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, storage.ErrGroupDeploymentRequestNotFound
		}
		return nil, err
	}
	return item, nil
}

func (s *GroupDeploymentRequestStore) Review(ctx context.Context, id string, from, to configs.GroupDeploymentRequestStatus, reviewer, comment string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `UPDATE group_deployment_requests
        SET status=?,reviewed_by=?,review_comment=?,reviewed_at=? WHERE id=? AND status=?`,
		string(to), reviewer, comment, formatTime(now), id, string(from))
	if err != nil {
		return fmt.Errorf("review group deployment request: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return storage.ErrGroupDeploymentRequestConflict
	}
	return nil
}

func (s *GroupDeploymentRequestStore) UpdateStatus(ctx context.Context, id string, from, to configs.GroupDeploymentRequestStatus) error {
	result, err := s.db.ExecContext(ctx, `UPDATE group_deployment_requests SET status=? WHERE id=? AND status=?`, string(to), id, string(from))
	if err != nil {
		return fmt.Errorf("update group deployment request status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read group deployment request update count: %w", err)
	}
	if affected != 1 {
		return storage.ErrGroupDeploymentRequestConflict
	}
	return nil
}

func (s *GroupDeploymentRequestStore) ListByGroup(ctx context.Context, groupID string, limit int) ([]*configs.GroupDeploymentRequest, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, groupDeploymentRequestSelect+` WHERE group_id=? ORDER BY created_at DESC LIMIT ?`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("list group deployment requests: %w", err)
	}
	defer rows.Close()
	result := make([]*configs.GroupDeploymentRequest, 0)
	for rows.Next() {
		item, err := scanGroupDeploymentRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *GroupDeploymentRequestStore) List(ctx context.Context, limit int) ([]*configs.GroupDeploymentRequest, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, groupDeploymentRequestSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list group deployment requests: %w", err)
	}
	defer rows.Close()
	result := make([]*configs.GroupDeploymentRequest, 0)
	for rows.Next() {
		item, err := scanGroupDeploymentRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

const groupDeploymentRequestSelect = `SELECT id,group_id,group_name,group_selector,configuration_id,configuration_name,configuration_version,configuration_hash,base_configuration_id,base_configuration_hash,targets,requested_by,reviewed_by,review_comment,reviewed_at,status,created_at FROM group_deployment_requests`

type groupDeploymentRequestScanner interface {
	Scan(...any) error
}

func scanGroupDeploymentRequest(scanner groupDeploymentRequestScanner) (*configs.GroupDeploymentRequest, error) {
	var item configs.GroupDeploymentRequest
	var selector, targets, status, created string
	var reviewedAt sql.NullString
	if err := scanner.Scan(&item.ID, &item.GroupID, &item.GroupName, &selector, &item.ConfigurationID, &item.ConfigurationName, &item.ConfigurationVersion, &item.ConfigurationHash, &item.BaseConfigurationID, &item.BaseConfigurationHash, &targets, &item.RequestedBy, &item.ReviewedBy, &item.ReviewComment, &reviewedAt, &status, &created); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(selector), &item.GroupSelector); err != nil {
		return nil, fmt.Errorf("decode group selector: %w", err)
	}
	if err := json.Unmarshal([]byte(targets), &item.Targets); err != nil {
		return nil, fmt.Errorf("decode deployment targets: %w", err)
	}
	item.Status = configs.GroupDeploymentRequestStatus(status)
	createdAt, err := parseTime(created)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = createdAt
	if item.ReviewedAt, err = parseNullTime(reviewedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

var _ storage.GroupDeploymentRequestStore = (*GroupDeploymentRequestStore)(nil)
