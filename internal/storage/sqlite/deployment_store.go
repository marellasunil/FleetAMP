// SQLite-backed append-only FleetAMP deployment history store.
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

type DeploymentStore struct{ db *sql.DB }

// Create inserts an immutable deployment attempt before transport delivery begins.
func (s *DeploymentStore) Create(ctx context.Context, d *configs.Deployment) error {
	if d == nil {
		return fmt.Errorf("deployment is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO deployments
        (id,agent_instance_uid,configuration_id,configuration_name,configuration_version,configuration_hash,action,status,error,created_at,sent_at,applying_at,applied_at,failed_at,updated_at)
        VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, d.ID, d.AgentInstanceUID, d.ConfigurationID,
		d.ConfigurationName, d.ConfigurationVersion, d.ConfigurationHash, string(d.Action), string(d.Status), d.Error,
		formatTime(d.CreatedAt), formatTimePtr(d.SentAt), formatTimePtr(d.ApplyingAt), formatTimePtr(d.AppliedAt), formatTimePtr(d.FailedAt), formatTime(d.UpdatedAt))
	if err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}
	return nil
}

// Get loads one deployment by its unique identifier.
func (s *DeploymentStore) Get(ctx context.Context, id string) (*configs.Deployment, error) {
	row := s.db.QueryRowContext(ctx, deploymentSelect+` WHERE id=?`, id)
	return scanDeployment(row)
}

// ListByAgent returns the newest deployment attempts for one agent, optionally limited.
func (s *DeploymentStore) ListByAgent(ctx context.Context, agentUID string, limit int) ([]*configs.Deployment, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, deploymentSelect+` WHERE agent_instance_uid=? ORDER BY created_at DESC LIMIT ?`, agentUID, limit)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()
	result := make([]*configs.Deployment, 0)
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	return result, nil
}

// UpdateLatestByAgentHash applies an OpAMP status report to the newest matching deployment attempt.
func (s *DeploymentStore) UpdateLatestByAgentHash(ctx context.Context, agentUID, configHash string, status configs.DeliveryStatus, errText string) error {
	now := time.Now().UTC()
	var sent, applying, applied, failed any
	switch status {
	case configs.DeliverySent:
		sent = formatTime(now)
	case configs.DeliveryApplying:
		applying = formatTime(now)
	case configs.DeliveryApplied:
		applied = formatTime(now)
	case configs.DeliveryFailed, configs.DeliveryUnsupported:
		failed = formatTime(now)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE deployments SET status=?, error=?, updated_at=?,
        sent_at=COALESCE(?,sent_at), applying_at=COALESCE(?,applying_at), applied_at=COALESCE(?,applied_at), failed_at=COALESCE(?,failed_at)
        WHERE id=(SELECT id FROM deployments WHERE agent_instance_uid=? AND configuration_hash=? ORDER BY created_at DESC LIMIT 1)`,
		string(status), errText, formatTime(now), sent, applying, applied, failed, agentUID, configHash)
	if err != nil {
		return fmt.Errorf("update deployment status: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deployment update result: %w", err)
	}
	if changed == 0 {
		return storage.ErrDeploymentNotFound
	}
	return nil
}

const deploymentSelect = `SELECT id,agent_instance_uid,configuration_id,configuration_name,configuration_version,configuration_hash,action,status,error,created_at,sent_at,applying_at,applied_at,failed_at,updated_at FROM deployments`

type deploymentScanner interface{ Scan(dest ...any) error }

// scanDeployment converts a SQL row, including nullable timestamps, into the deployment model.
func scanDeployment(scanner deploymentScanner) (*configs.Deployment, error) {
	var d configs.Deployment
	var action, status, created, updated string
	var sent, applying, applied, failed sql.NullString
	if err := scanner.Scan(&d.ID, &d.AgentInstanceUID, &d.ConfigurationID, &d.ConfigurationName, &d.ConfigurationVersion, &d.ConfigurationHash, &action, &status, &d.Error, &created, &sent, &applying, &applied, &failed, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, storage.ErrDeploymentNotFound
		}
		return nil, fmt.Errorf("read deployment: %w", err)
	}
	d.Action, d.Status = configs.DeploymentAction(action), configs.DeliveryStatus(status)
	var err error
	if d.CreatedAt, err = parseTime(created); err != nil {
		return nil, err
	}
	if d.UpdatedAt, err = parseTime(updated); err != nil {
		return nil, err
	}
	if d.SentAt, err = parseNullTime(sent); err != nil {
		return nil, err
	}
	if d.ApplyingAt, err = parseNullTime(applying); err != nil {
		return nil, err
	}
	if d.AppliedAt, err = parseNullTime(applied); err != nil {
		return nil, err
	}
	if d.FailedAt, err = parseNullTime(failed); err != nil {
		return nil, err
	}
	return &d, nil
}

// formatTime serializes timestamps consistently at nanosecond precision for SQLite.
func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

// formatTimePtr converts an optional timestamp into a nullable SQL value.
func formatTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

// parseTime restores a required RFC3339-nanosecond timestamp from SQLite.
func parseTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse deployment timestamp: %w", err)
	}
	return t, nil
}

// parseNullTime restores an optional timestamp from a nullable SQL column.
func parseNullTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	t, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

var _ storage.DeploymentStore = (*DeploymentStore)(nil)
