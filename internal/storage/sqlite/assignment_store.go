// SQLite-backed implementation of FleetAMP AssignmentStore.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type AssignmentStore struct{ db *sql.DB }

func (s *AssignmentStore) Upsert(ctx context.Context, a *configs.Assignment) error {
	if a == nil {
		return fmt.Errorf("assignment is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO assignments
        (agent_instance_uid,configuration_id,configuration_hash,status,error,updated_at)
        VALUES(?,?,?,?,?,?) ON CONFLICT(agent_instance_uid,configuration_id) DO UPDATE SET
        configuration_hash=excluded.configuration_hash,status=excluded.status,error=excluded.error,updated_at=excluded.updated_at`,
		a.AgentInstanceUID, a.ConfigurationID, a.ConfigurationHash, string(a.Status), a.Error, a.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("store assignment: %w", err)
	}
	return nil
}

func (s *AssignmentStore) Get(ctx context.Context, agentUID, configID string) (*configs.Assignment, error) {
	row := s.db.QueryRowContext(ctx, `SELECT agent_instance_uid,configuration_id,configuration_hash,status,error,updated_at
        FROM assignments WHERE agent_instance_uid=? AND configuration_id=?`, agentUID, configID)
	return scanAssignment(row)
}

func (s *AssignmentStore) List(ctx context.Context) ([]*configs.Assignment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT agent_instance_uid,configuration_id,configuration_hash,status,error,updated_at
        FROM assignments ORDER BY updated_at DESC, agent_instance_uid, configuration_id`)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	defer rows.Close()
	result := make([]*configs.Assignment, 0)
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	return result, nil
}

func (s *AssignmentStore) UpdateByAgentHash(ctx context.Context, agentUID, configHash string, status configs.DeliveryStatus, errText string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `UPDATE assignments SET status=?, error=?, updated_at=? WHERE rowid=(
        SELECT rowid FROM assignments WHERE agent_instance_uid=? AND configuration_hash=? ORDER BY updated_at DESC LIMIT 1)`,
		string(status), errText, now, agentUID, configHash)
	if err != nil {
		return fmt.Errorf("update assignment status: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read assignment update result: %w", err)
	}
	if changed == 0 {
		return storage.ErrAssignmentNotFound
	}
	return nil
}

type assignmentScanner interface{ Scan(dest ...any) error }

func scanAssignment(scanner assignmentScanner) (*configs.Assignment, error) {
	var a configs.Assignment
	var status, updated string
	if err := scanner.Scan(&a.AgentInstanceUID, &a.ConfigurationID, &a.ConfigurationHash, &status, &a.Error, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrAssignmentNotFound
		}
		return nil, fmt.Errorf("read assignment: %w", err)
	}
	a.Status = configs.DeliveryStatus(status)
	parsed, err := time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		return nil, fmt.Errorf("parse assignment updated_at: %w", err)
	}
	a.UpdatedAt = parsed
	return &a, nil
}

var _ storage.AssignmentStore = (*AssignmentStore)(nil)
