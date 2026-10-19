package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildSourceInspectionStore struct{ db *sql.DB }

func (s *DistributionBuildSourceInspectionStore) Create(ctx context.Context, item *distributions.BuildSourceInspection) error {
	if item == nil {
		return fmt.Errorf("source inspection is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_source_inspections(id,acquisition_id,execution_id,build_plan_hash,archive_sha256,entry_count,file_count,directory_count,uncompressed_bytes,root_prefix,manifest_yaml,manifest_sha256,status,evidence_hash,inspected_by,inspected_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.AcquisitionID, item.ExecutionID, item.BuildPlanHash, item.ArchiveSHA256, item.EntryCount, item.FileCount, item.DirectoryCount, item.UncompressedBytes, item.RootPrefix, item.ManifestYAML, item.ManifestSHA256, item.Status, item.EvidenceHash, item.InspectedBy, item.InspectedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create source inspection: %w", err)
	}
	return nil
}
func (s *DistributionBuildSourceInspectionStore) List(ctx context.Context, limit int) ([]*distributions.BuildSourceInspection, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,acquisition_id,execution_id,build_plan_hash,archive_sha256,entry_count,file_count,directory_count,uncompressed_bytes,root_prefix,manifest_yaml,manifest_sha256,status,evidence_hash,inspected_by,inspected_at FROM distribution_build_source_inspections ORDER BY inspected_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildSourceInspection
	for rows.Next() {
		item := &distributions.BuildSourceInspection{}
		var at string
		if err := rows.Scan(&item.ID, &item.AcquisitionID, &item.ExecutionID, &item.BuildPlanHash, &item.ArchiveSHA256, &item.EntryCount, &item.FileCount, &item.DirectoryCount, &item.UncompressedBytes, &item.RootPrefix, &item.ManifestYAML, &item.ManifestSHA256, &item.Status, &item.EvidenceHash, &item.InspectedBy, &at); err != nil {
			return nil, err
		}
		item.InspectedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}
