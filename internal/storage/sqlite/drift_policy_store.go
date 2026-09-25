// SQLite-backed global configuration drift policy.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
)

type DriftPolicyStore struct{ db *sql.DB }

func (s *DriftPolicyStore) Get(ctx context.Context) (configs.DriftPolicy, error) {
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT policy FROM configuration_drift_policy WHERE singleton=1`).Scan(&value); err != nil {
		return "", fmt.Errorf("read configuration drift policy: %w", err)
	}
	return configs.ParseDriftPolicy(value)
}

func (s *DriftPolicyStore) Set(ctx context.Context, policy configs.DriftPolicy) error {
	if !policy.Valid() {
		return fmt.Errorf("invalid configuration drift policy")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE configuration_drift_policy SET policy=?,updated_at=? WHERE singleton=1`,
		string(policy), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("update configuration drift policy: %w", err)
	}
	return nil
}
