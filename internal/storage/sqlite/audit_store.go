// SQLite-backed append-only FleetAMP audit event store.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/marellasunil/FleetAMP/internal/audit"
)

type AuditStore struct{ db *sql.DB }

func (s *AuditStore) Append(ctx context.Context, event *audit.Event) error {
	if event == nil {
		return fmt.Errorf("audit event is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO audit_events
        (occurred_at,actor,action,resource_type,resource_id,outcome,http_method,path,status_code)
        VALUES(?,?,?,?,?,?,?,?,?)`,
		event.Timestamp.UTC().Format(time.RFC3339Nano), event.Actor, event.Action,
		event.ResourceType, event.ResourceID, event.Outcome, event.HTTPMethod,
		event.Path, event.StatusCode)
	if err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	event.ID, _ = result.LastInsertId()
	return nil
}
func (s *AuditStore) List(ctx context.Context, filter audit.Filter) ([]*audit.Event, error) {
	filter = filter.Normalized()
	query := `SELECT id,occurred_at,actor,action,resource_type,resource_id,
        outcome,http_method,path,status_code FROM audit_events WHERE 1=1`
	args := make([]any, 0, 4)
	if filter.Actor != "" {
		query += " AND actor=?"
		args = append(args, filter.Actor)
	}
	if filter.Action != "" {
		query += " AND action=?"
		args = append(args, filter.Action)
	}
	if filter.Outcome != "" {
		query += " AND outcome=?"
		args = append(args, filter.Outcome)
	}
	if !filter.Since.IsZero() {
		query += " AND occurred_at>=?"
		args = append(args, filter.Since.UTC().Format(time.RFC3339Nano))
	}
	if !filter.Until.IsZero() {
		query += " AND occurred_at<?"
		args = append(args, filter.Until.UTC().Format(time.RFC3339Nano))
	}
	query += " ORDER BY occurred_at DESC,id DESC LIMIT ?"
	args = append(args, filter.Limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()
	events := make([]*audit.Event, 0)
	for rows.Next() {
		event := &audit.Event{}
		var occurred string
		if err := rows.Scan(&event.ID, &occurred, &event.Actor, &event.Action,
			&event.ResourceType, &event.ResourceID, &event.Outcome,
			&event.HTTPMethod, &event.Path, &event.StatusCode); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		event.Timestamp, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(occurred))
		if err != nil {
			return nil, fmt.Errorf("parse audit timestamp: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return events, nil
}
