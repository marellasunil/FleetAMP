package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/marellasunil/FleetAMP/internal/lifecycle"
	"github.com/marellasunil/FleetAMP/internal/runtimes"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type ComponentLifecycleRequestStore struct{ db *sql.DB }

func (s *ComponentLifecycleRequestStore) Create(ctx context.Context, request *lifecycle.Request) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO component_lifecycle_requests
		(id,operation,component_type,group_id,label_selector,deployment_method,current_version,desired_version,reason,spec_hash,requested_by,status,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, request.ID, string(request.Spec.Operation), string(request.Spec.ComponentType), request.Spec.GroupID,
		request.Spec.LabelSelector, request.Spec.DeploymentMethod, request.Spec.CurrentVersion, request.Spec.DesiredVersion, request.Spec.Reason,
		request.SpecHash, request.RequestedBy, string(request.Status), formatTime(request.CreatedAt))
	if err != nil {
		return fmt.Errorf("create component lifecycle request: %w", err)
	}
	return nil
}

func (s *ComponentLifecycleRequestStore) Get(ctx context.Context, id string) (*lifecycle.Request, error) {
	request, err := scanComponentLifecycleRequest(s.db.QueryRowContext(ctx, componentLifecycleRequestSelect+` WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, storage.ErrComponentLifecycleRequestNotFound
	}
	return request, err
}

func (s *ComponentLifecycleRequestStore) List(ctx context.Context, limit int) ([]*lifecycle.Request, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, componentLifecycleRequestSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list component lifecycle requests: %w", err)
	}
	defer rows.Close()
	result := make([]*lifecycle.Request, 0)
	for rows.Next() {
		request, err := scanComponentLifecycleRequest(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, request)
	}
	return result, rows.Err()
}

const componentLifecycleRequestSelect = `SELECT id,operation,component_type,group_id,label_selector,deployment_method,current_version,desired_version,reason,spec_hash,requested_by,status,created_at FROM component_lifecycle_requests`

type componentLifecycleRequestScanner interface{ Scan(...any) error }

func scanComponentLifecycleRequest(scanner componentLifecycleRequestScanner) (*lifecycle.Request, error) {
	var request lifecycle.Request
	var operation, componentType, status, createdAt string
	if err := scanner.Scan(&request.ID, &operation, &componentType, &request.Spec.GroupID, &request.Spec.LabelSelector,
		&request.Spec.DeploymentMethod, &request.Spec.CurrentVersion, &request.Spec.DesiredVersion, &request.Spec.Reason,
		&request.SpecHash, &request.RequestedBy, &status, &createdAt); err != nil {
		return nil, err
	}
	request.Spec.Operation = lifecycle.Operation(operation)
	request.Spec.ComponentType = runtimes.Type(componentType)
	request.Status = lifecycle.Status(status)
	parsed, err := parseTime(createdAt)
	if err != nil {
		return nil, err
	}
	request.CreatedAt = parsed
	return &request, nil
}

var _ storage.ComponentLifecycleRequestStore = (*ComponentLifecycleRequestStore)(nil)
