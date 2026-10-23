package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/marellasunil/FleetAMP/internal/configs"
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

func (s *DistributionPreviewHandoffStore) CreateApprovalHandoff(ctx context.Context, configuration *configs.Configuration, request *configs.GroupDeploymentRequest, handoff *distributions.PreviewHandoff) error {
	if configuration == nil || request == nil || handoff == nil {
		return fmt.Errorf("configuration, approval request and distribution preview handoff are required")
	}
	if request.ConfigurationID != configuration.ID || request.ConfigurationHash != configuration.Hash || handoff.ConfigurationID != configuration.ID || handoff.ConfigurationHash != configuration.Hash || handoff.ApprovalRequestID != request.ID || handoff.GroupID != configuration.GroupID || request.GroupID != configuration.GroupID {
		return fmt.Errorf("distribution handoff evidence does not match configuration and approval request")
	}
	selector, err := json.Marshal(request.GroupSelector)
	if err != nil {
		return fmt.Errorf("encode group selector: %w", err)
	}
	labelSelector, err := json.Marshal(request.LabelSelector)
	if err != nil {
		return fmt.Errorf("encode label selector: %w", err)
	}
	targets, err := json.Marshal(request.Targets)
	if err != nil {
		return fmt.Errorf("encode deployment targets: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin distribution approval handoff: %w", err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO configurations(id,group_id,name,version,content,content_type,hash,created_at) VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, configuration.ID, configuration.GroupID, configuration.Name, configuration.Version, configuration.Content, configuration.ContentType, configuration.Hash, formatTime(configuration.CreatedAt)); err != nil {
		return fmt.Errorf("store handoff configuration: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO group_deployment_requests(id,group_id,group_name,group_selector,label_selector,configuration_id,configuration_name,configuration_version,configuration_hash,base_configuration_id,base_configuration_hash,targets,requested_by,assigned_reviewer,change_reason,reviewed_by,review_comment,reviewed_at,expires_at,expired_at,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, request.ID, request.GroupID, request.GroupName, string(selector), string(labelSelector), request.ConfigurationID, request.ConfigurationName, request.ConfigurationVersion, request.ConfigurationHash, request.BaseConfigurationID, request.BaseConfigurationHash, string(targets), request.RequestedBy, request.AssignedReviewer, request.ChangeReason, request.ReviewedBy, request.ReviewComment, formatTimePtr(request.ReviewedAt), formatTime(request.ExpiresAt), formatTimePtr(request.ExpiredAt), string(request.Status), formatTime(request.CreatedAt)); err != nil {
		return fmt.Errorf("store handoff approval request: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO distribution_preview_handoffs(id,preview_id,preview_content_hash,release_id,release_evidence_hash,artifact_sha256,group_id,configuration_id,configuration_hash,approval_request_id,submitted_by,evidence_hash,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, handoff.ID, handoff.PreviewID, handoff.PreviewContentHash, handoff.ReleaseID, handoff.ReleaseEvidenceHash, handoff.ArtifactSHA256, handoff.GroupID, handoff.ConfigurationID, handoff.ConfigurationHash, handoff.ApprovalRequestID, handoff.SubmittedBy, handoff.EvidenceHash, formatTime(handoff.CreatedAt)); err != nil {
		return fmt.Errorf("store distribution handoff receipt: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit distribution approval handoff: %w", err)
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
