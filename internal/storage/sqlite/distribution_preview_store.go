package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionPreviewStore struct{ db *sql.DB }

func (s *DistributionPreviewStore) Create(ctx context.Context, p *distributions.DeploymentPreview) error {
	sections, _ := json.Marshal(p.Sections)
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_deployment_previews(id,distribution_id,distribution_spec_hash,release_id,release_evidence_hash,artifact_uri,artifact_sha256,group_id,group_name,sections,composed_config,content_hash,validation_evidence,prepared_by,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, p.ID, p.DistributionID, p.DistributionSpecHash, p.ReleaseID, p.ReleaseEvidenceHash, p.ArtifactURI, p.ArtifactSHA256, p.GroupID, p.GroupName, string(sections), p.ComposedConfig, p.ContentHash, p.ValidationEvidence, p.PreparedBy, p.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution preview: %w", err)
	}
	return nil
}
func scanDistributionPreview(row interface{ Scan(...any) error }) (*distributions.DeploymentPreview, error) {
	p := &distributions.DeploymentPreview{}
	var sections, created string
	if err := row.Scan(&p.ID, &p.DistributionID, &p.DistributionSpecHash, &p.ReleaseID, &p.ReleaseEvidenceHash, &p.ArtifactURI, &p.ArtifactSHA256, &p.GroupID, &p.GroupName, &sections, &p.ComposedConfig, &p.ContentHash, &p.ValidationEvidence, &p.PreparedBy, &created); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(sections), &p.Sections); err != nil {
		return nil, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return p, nil
}
func (s *DistributionPreviewStore) Get(ctx context.Context, id string) (*distributions.DeploymentPreview, error) {
	return scanDistributionPreview(s.db.QueryRowContext(ctx, `SELECT id,distribution_id,distribution_spec_hash,release_id,release_evidence_hash,artifact_uri,artifact_sha256,group_id,group_name,sections,composed_config,content_hash,validation_evidence,prepared_by,created_at FROM distribution_deployment_previews WHERE id=?`, id))
}
func (s *DistributionPreviewStore) List(ctx context.Context, limit int) ([]*distributions.DeploymentPreview, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,distribution_id,distribution_spec_hash,release_id,release_evidence_hash,artifact_uri,artifact_sha256,group_id,group_name,sections,composed_config,content_hash,validation_evidence,prepared_by,created_at FROM distribution_deployment_previews ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.DeploymentPreview
	for rows.Next() {
		p, e := scanDistributionPreview(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
