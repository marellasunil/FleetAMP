package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionBuildArtifactStore struct{ db *sql.DB }

func (s *DistributionBuildArtifactStore) Create(ctx context.Context, item *distributions.BuildArtifact) error {
	if item == nil {
		return fmt.Errorf("build artifact is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_build_artifacts(id,inspection_id,execution_id,build_plan_hash,manifest_sha256,runtime,builder_image,artifact_reference,artifact_sha256,artifact_size,status,evidence_hash,built_by,built_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.InspectionID, item.ExecutionID, item.BuildPlanHash, item.ManifestSHA256, item.Runtime, item.BuilderImage, item.ArtifactReference, item.ArtifactSHA256, item.ArtifactSize, item.Status, item.EvidenceHash, item.BuiltBy, item.BuiltAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution build artifact: %w", err)
	}
	return nil
}
func (s *DistributionBuildArtifactStore) List(ctx context.Context, limit int) ([]*distributions.BuildArtifact, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,inspection_id,execution_id,build_plan_hash,manifest_sha256,runtime,builder_image,artifact_reference,artifact_sha256,artifact_size,status,evidence_hash,built_by,built_at FROM distribution_build_artifacts ORDER BY built_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.BuildArtifact
	for rows.Next() {
		item := &distributions.BuildArtifact{}
		var at string
		if err := rows.Scan(&item.ID, &item.InspectionID, &item.ExecutionID, &item.BuildPlanHash, &item.ManifestSHA256, &item.Runtime, &item.BuilderImage, &item.ArtifactReference, &item.ArtifactSHA256, &item.ArtifactSize, &item.Status, &item.EvidenceHash, &item.BuiltBy, &at); err != nil {
			return nil, err
		}
		item.BuiltAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}
