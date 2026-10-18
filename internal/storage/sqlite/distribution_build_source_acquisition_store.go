package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildSourceAcquisitionStore struct{ db *sql.DB }

func (s *DistributionBuildSourceAcquisitionStore) Create(ctx context.Context, item *distributions.BuildSourceAcquisition) error {
	if item == nil {
		return fmt.Errorf("source acquisition is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_source_acquisitions(id,source_resolution_id,execution_id,build_plan_hash,archive_sha256,archive_size,local_reference,status,evidence_hash,acquired_by,acquired_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.SourceResolutionID, item.ExecutionID, item.BuildPlanHash, item.ArchiveSHA256, item.ArchiveSize, item.LocalReference, item.Status, item.EvidenceHash, item.AcquiredBy, item.AcquiredAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create source acquisition: %w", err)
	}
	return nil
}
func (s *DistributionBuildSourceAcquisitionStore) List(ctx context.Context, limit int) ([]*distributions.BuildSourceAcquisition, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,source_resolution_id,execution_id,build_plan_hash,archive_sha256,archive_size,local_reference,status,evidence_hash,acquired_by,acquired_at FROM distribution_build_source_acquisitions ORDER BY acquired_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildSourceAcquisition
	for rows.Next() {
		item := &distributions.BuildSourceAcquisition{}
		var acquiredAt string
		if err := rows.Scan(&item.ID, &item.SourceResolutionID, &item.ExecutionID, &item.BuildPlanHash, &item.ArchiveSHA256, &item.ArchiveSize, &item.LocalReference, &item.Status, &item.EvidenceHash, &item.AcquiredBy, &acquiredAt); err != nil {
			return nil, err
		}
		item.AcquiredAt, _ = time.Parse(time.RFC3339Nano, acquiredAt)
		out = append(out, item)
	}
	return out, rows.Err()
}
