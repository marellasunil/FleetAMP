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

type ComponentLifecycleValidationStore struct{ db *sql.DB }

func (s *ComponentLifecycleValidationStore) Create(ctx context.Context, value *lifecycle.Validation) error {
	groupSelector, err := json.Marshal(value.GroupSelector); if err != nil { return err }
	labelSelector, err := json.Marshal(value.LabelSelector); if err != nil { return err }
	targets, err := json.Marshal(value.Targets); if err != nil { return err }
	findings, err := json.Marshal(value.Findings); if err != nil { return err }
	_, err = s.db.ExecContext(ctx, `INSERT INTO component_lifecycle_validations
		(id,request_id,request_spec_hash,group_id,group_name,group_selector,label_selector,targets,findings,status,result_hash,validated_by,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.RequestID, value.RequestSpecHash, value.GroupID, value.GroupName,
		string(groupSelector), string(labelSelector), string(targets), string(findings), value.Status, value.ResultHash, value.ValidatedBy, value.CreatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil { return fmt.Errorf("create component lifecycle validation: %w", err) }
	return nil
}

func (s *ComponentLifecycleValidationStore) Get(ctx context.Context, id string) (*lifecycle.Validation, error) {
	value, err := scanComponentLifecycleValidation(s.db.QueryRowContext(ctx, componentLifecycleValidationSelect+` WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) { return nil, storage.ErrComponentLifecycleValidationNotFound }
	return value, err
}

func (s *ComponentLifecycleValidationStore) ListByRequest(ctx context.Context, requestID string) ([]*lifecycle.Validation, error) {
	rows, err := s.db.QueryContext(ctx, componentLifecycleValidationSelect+` WHERE request_id=? ORDER BY created_at DESC`, requestID)
	if err != nil { return nil, fmt.Errorf("list component lifecycle validations: %w", err) }
	defer rows.Close()
	result := []*lifecycle.Validation{}
	for rows.Next() { value, err := scanComponentLifecycleValidation(rows); if err != nil { return nil, err }; result = append(result, value) }
	return result, rows.Err()
}

const componentLifecycleValidationSelect = `SELECT id,request_id,request_spec_hash,group_id,group_name,group_selector,label_selector,targets,findings,status,result_hash,validated_by,created_at FROM component_lifecycle_validations`
type componentLifecycleValidationScanner interface{ Scan(...any) error }
func scanComponentLifecycleValidation(scanner componentLifecycleValidationScanner) (*lifecycle.Validation, error) {
	value := &lifecycle.Validation{}
	var groupSelector, labelSelector, targets, findings, status, createdAt string
	if err := scanner.Scan(&value.ID, &value.RequestID, &value.RequestSpecHash, &value.GroupID, &value.GroupName, &groupSelector, &labelSelector, &targets, &findings, &status, &value.ResultHash, &value.ValidatedBy, &createdAt); err != nil { return nil, err }
	if err := json.Unmarshal([]byte(groupSelector), &value.GroupSelector); err != nil { return nil, err }
	if err := json.Unmarshal([]byte(labelSelector), &value.LabelSelector); err != nil { return nil, err }
	if err := json.Unmarshal([]byte(targets), &value.Targets); err != nil { return nil, err }
	if err := json.Unmarshal([]byte(findings), &value.Findings); err != nil { return nil, err }
	value.Status = lifecycle.ValidationStatus(status)
	value.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return value, nil
}
var _ storage.ComponentLifecycleValidationStore = (*ComponentLifecycleValidationStore)(nil)
