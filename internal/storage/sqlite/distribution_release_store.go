package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionReleaseStore struct{ db *sql.DB }

func (s *DistributionReleaseStore) Create(ctx context.Context, item *distributions.DistributionRelease) error {
	if item == nil {
		return fmt.Errorf("distribution release is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_releases(id,distribution_id,distribution_spec_hash,build_plan_id,build_plan_hash,artifact_id,artifact_evidence_hash,artifact_sha256,publication_id,publication_evidence_hash,execution_id,execution_evidence_hash,artifact_uri,released_by,evidence_hash,released_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.DistributionID, item.DistributionSpecHash, item.BuildPlanID, item.BuildPlanHash, item.ArtifactID, item.ArtifactEvidenceHash, item.ArtifactSHA256, item.PublicationID, item.PublicationEvidenceHash, item.ExecutionID, item.ExecutionEvidenceHash, item.ArtifactURI, item.ReleasedBy, item.EvidenceHash, formatTime(item.ReleasedAt))
	if err != nil {
		return fmt.Errorf("create distribution release: %w", err)
	}
	return nil
}

func (s *DistributionReleaseStore) List(ctx context.Context, limit int) ([]*distributions.DistributionRelease, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,distribution_id,distribution_spec_hash,build_plan_id,build_plan_hash,artifact_id,artifact_evidence_hash,artifact_sha256,publication_id,publication_evidence_hash,execution_id,execution_evidence_hash,artifact_uri,released_by,evidence_hash,released_at FROM distribution_releases ORDER BY released_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.DistributionRelease
	for rows.Next() {
		item := &distributions.DistributionRelease{}
		var at string
		if err := rows.Scan(&item.ID, &item.DistributionID, &item.DistributionSpecHash, &item.BuildPlanID, &item.BuildPlanHash, &item.ArtifactID, &item.ArtifactEvidenceHash, &item.ArtifactSHA256, &item.PublicationID, &item.PublicationEvidenceHash, &item.ExecutionID, &item.ExecutionEvidenceHash, &item.ArtifactURI, &item.ReleasedBy, &item.EvidenceHash, &at); err != nil {
			return nil, err
		}
		item.ReleasedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}
