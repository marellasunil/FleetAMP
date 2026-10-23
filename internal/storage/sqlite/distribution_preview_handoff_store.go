package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"time"
)

type DistributionPreviewHandoffStore struct{ db *sql.DB }

func (s *DistributionPreviewHandoffStore) Create(ctx context.Context, h *distributions.PreviewHandoff) error {
	if h == nil {
		return fmt.Errorf("distribution preview handoff is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_preview_handoffs(id,preview_id,preview_content_hash,release_id,release_evidence_hash,artifact_sha256,group_id,configuration_id,configuration_hash,approval_request_id,submitted_by,evidence_hash,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, h.ID, h.PreviewID, h.PreviewContentHash, h.ReleaseID, h.ReleaseEvidenceHash, h.ArtifactSHA256, h.GroupID, h.ConfigurationID, h.ConfigurationHash, h.ApprovalRequestID, h.SubmittedBy, h.EvidenceHash, h.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create distribution preview handoff: %w", err)
	}
	return nil
}
func scanDistributionPreviewHandoff(row interface{ Scan(...any) error }) (*distributions.PreviewHandoff, error) {
	h := &distributions.PreviewHandoff{}
	var created string
	if err := row.Scan(&h.ID, &h.PreviewID, &h.PreviewContentHash, &h.ReleaseID, &h.ReleaseEvidenceHash, &h.ArtifactSHA256, &h.GroupID, &h.ConfigurationID, &h.ConfigurationHash, &h.ApprovalRequestID, &h.SubmittedBy, &h.EvidenceHash, &created); err != nil {
		return nil, err
	}
	h.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return h, nil
}

const distributionPreviewHandoffSelect = `SELECT id,preview_id,preview_content_hash,release_id,release_evidence_hash,artifact_sha256,group_id,configuration_id,configuration_hash,approval_request_id,submitted_by,evidence_hash,created_at FROM distribution_preview_handoffs`

func (s *DistributionPreviewHandoffStore) GetByPreview(ctx context.Context, id string) (*distributions.PreviewHandoff, error) {
	return scanDistributionPreviewHandoff(s.db.QueryRowContext(ctx, distributionPreviewHandoffSelect+` WHERE preview_id=?`, id))
}
func (s *DistributionPreviewHandoffStore) List(ctx context.Context, limit int) ([]*distributions.PreviewHandoff, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, distributionPreviewHandoffSelect+` ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.PreviewHandoff
	for rows.Next() {
		h, e := scanDistributionPreviewHandoff(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
