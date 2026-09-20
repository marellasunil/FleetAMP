[Reading 71 lines from start (total: 71 lines, 0 remaining)]

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

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
        (id,group_id,group_name,group_selector,configuration_id,configuration_name,configuration_version,configuration_hash,targets,requested_by,status,created_at)
        VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, request.ID, request.GroupID, request.GroupName, string(selector),
		request.ConfigurationID, request.ConfigurationName, request.ConfigurationVersion, request.ConfigurationHash,
		string(targets), request.RequestedBy, string(request.Status), formatTime(request.CreatedAt))
	if err != nil {
		return fmt.Errorf("create group deployment request: %w", err)
	}
	return nil
}

func (s *GroupDeploymentRequestStore) ListByGroup(ctx context.Context, groupID string, limit int) ([]*configs.GroupDeploymentRequest, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,group_id,group_name,group_selector,configuration_id,configuration_name,configuration_version,configuration_hash,targets,requested_by,status,created_at
        FROM group_deployment_requests WHERE group_id=? ORDER BY created_at DESC LIMIT ?`, groupID, limit)
	if err != nil {
		return nil, fmt.Errorf("list group deployment requests: %w", err)
	}
	defer rows.Close()
	result := make([]*configs.GroupDeploymentRequest, 0)
	for rows.Next() {
		var item configs.GroupDeploymentRequest
		var selector, targets, status, created string
		if err := rows.Scan(&item.ID, &item.GroupID, &item.GroupName, &selector, &item.ConfigurationID, &item.ConfigurationName, &item.ConfigurationVersion, &item.ConfigurationHash, &targets, &item.RequestedBy, &status, &created); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(selector), &item.GroupSelector); err != nil {
			return nil, fmt.Errorf("decode group selector: %w", err)
		}
		if err := json.Unmarshal([]byte(targets), &item.Targets); err != nil {
			return nil, fmt.Errorf("decode deployment targets: %w", err)
		}
		item.Status = configs.GroupDeploymentRequestStatus(status)
		item.CreatedAt, err = parseTime(created)
		if err != nil {
			return nil, err
		}
		result = append(result, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

var _ storage.GroupDeploymentRequestStore = (*GroupDeploymentRequestStore)(nil)