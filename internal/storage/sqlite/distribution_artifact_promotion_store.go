package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

type DistributionArtifactPromotionStore struct{ db *sql.DB }

func (s *DistributionArtifactPromotionStore) Create(ctx context.Context, item *distributions.ArtifactPromotion) error {
	if item == nil {
		return fmt.Errorf("artifact promotion is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_artifact_promotions(id,artifact_id,artifact_sha256,build_plan_hash,destination_ref,credential_ref,reason,requested_by,status,evidence_hash,requested_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.ArtifactID, item.ArtifactSHA256, item.BuildPlanHash, item.DestinationRef, item.CredentialRef, item.Reason, item.RequestedBy, item.Status, item.EvidenceHash, item.RequestedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create artifact promotion: %w", err)
	}
	return nil
}

func (s *DistributionArtifactPromotionStore) List(ctx context.Context, limit int) ([]*distributions.ArtifactPromotion, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,artifact_id,artifact_sha256,build_plan_hash,destination_ref,credential_ref,reason,requested_by,status,evidence_hash,requested_at FROM distribution_artifact_promotions ORDER BY requested_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.ArtifactPromotion
	for rows.Next() {
		item := &distributions.ArtifactPromotion{}
		var at string
		if err := rows.Scan(&item.ID, &item.ArtifactID, &item.ArtifactSHA256, &item.BuildPlanHash, &item.DestinationRef, &item.CredentialRef, &item.Reason, &item.RequestedBy, &item.Status, &item.EvidenceHash, &at); err != nil {
			return nil, err
		}
		item.RequestedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}

type DistributionArtifactPromotionReviewStore struct{ db *sql.DB }

func (s *DistributionArtifactPromotionReviewStore) Create(ctx context.Context, item *distributions.ArtifactPromotionReview) error {
	if item == nil {
		return fmt.Errorf("artifact promotion review is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_artifact_promotion_reviews(id,promotion_id,promotion_evidence_hash,artifact_sha256,decision,reason,reviewed_by,evidence_hash,reviewed_at) VALUES(?,?,?,?,?,?,?,?,?)`, item.ID, item.PromotionID, item.PromotionEvidenceHash, item.ArtifactSHA256, item.Decision, item.Reason, item.ReviewedBy, item.EvidenceHash, item.ReviewedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create artifact promotion review: %w", err)
	}
	return nil
}

func (s *DistributionArtifactPromotionReviewStore) List(ctx context.Context, limit int) ([]*distributions.ArtifactPromotionReview, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,promotion_id,promotion_evidence_hash,artifact_sha256,decision,reason,reviewed_by,evidence_hash,reviewed_at FROM distribution_artifact_promotion_reviews ORDER BY reviewed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.ArtifactPromotionReview
	for rows.Next() {
		item := &distributions.ArtifactPromotionReview{}
		var at string
		if err := rows.Scan(&item.ID, &item.PromotionID, &item.PromotionEvidenceHash, &item.ArtifactSHA256, &item.Decision, &item.Reason, &item.ReviewedBy, &item.EvidenceHash, &at); err != nil {
			return nil, err
		}
		item.ReviewedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}

type DistributionArtifactPublicationStore struct{ db *sql.DB }

func (s *DistributionArtifactPublicationStore) Create(ctx context.Context, item *distributions.ArtifactPublication) error {
	if item == nil {
		return fmt.Errorf("artifact publication is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_artifact_publications(id,promotion_id,review_id,artifact_id,artifact_sha256,destination_ref,credential_ref,status,evidence_hash,queued_by,queued_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.PromotionID, item.ReviewID, item.ArtifactID, item.ArtifactSHA256, item.DestinationRef, item.CredentialRef, item.Status, item.EvidenceHash, item.QueuedBy, item.QueuedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create artifact publication: %w", err)
	}
	return nil
}

type DistributionArtifactPublicationPreflightStore struct{ db *sql.DB }

func (s *DistributionArtifactPublicationPreflightStore) Create(ctx context.Context, item *distributions.ArtifactPublicationPreflight) error {
	if item == nil {
		return fmt.Errorf("artifact publication preflight is required")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO distribution_artifact_publication_preflights(id,publication_id,publication_evidence_hash,artifact_id,artifact_reference,artifact_sha256,artifact_size,destination_ref,destination_host,credential_ref,credential_available,status,evidence_hash,checked_by,checked_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.PublicationID, item.PublicationEvidenceHash, item.ArtifactID, item.ArtifactReference, item.ArtifactSHA256, item.ArtifactSize, item.DestinationRef, item.DestinationHost, item.CredentialRef, item.CredentialAvailable, item.Status, item.EvidenceHash, item.CheckedBy, item.CheckedAt.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("create artifact publication preflight: %w", err)
	}
	return nil
}

func (s *DistributionArtifactPublicationPreflightStore) List(ctx context.Context, limit int) ([]*distributions.ArtifactPublicationPreflight, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,publication_id,publication_evidence_hash,artifact_id,artifact_reference,artifact_sha256,artifact_size,destination_ref,destination_host,credential_ref,credential_available,status,evidence_hash,checked_by,checked_at FROM distribution_artifact_publication_preflights ORDER BY checked_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.ArtifactPublicationPreflight
	for rows.Next() {
		item := &distributions.ArtifactPublicationPreflight{}
		var at string
		if err := rows.Scan(&item.ID, &item.PublicationID, &item.PublicationEvidenceHash, &item.ArtifactID, &item.ArtifactReference, &item.ArtifactSHA256, &item.ArtifactSize, &item.DestinationRef, &item.DestinationHost, &item.CredentialRef, &item.CredentialAvailable, &item.Status, &item.EvidenceHash, &item.CheckedBy, &at); err != nil {
			return nil, err
		}
		item.CheckedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *DistributionArtifactPublicationStore) List(ctx context.Context, limit int) ([]*distributions.ArtifactPublication, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,promotion_id,review_id,artifact_id,artifact_sha256,destination_ref,credential_ref,status,evidence_hash,queued_by,queued_at FROM distribution_artifact_publications ORDER BY queued_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.ArtifactPublication
	for rows.Next() {
		item := &distributions.ArtifactPublication{}
		var at string
		if err := rows.Scan(&item.ID, &item.PromotionID, &item.ReviewID, &item.ArtifactID, &item.ArtifactSHA256, &item.DestinationRef, &item.CredentialRef, &item.Status, &item.EvidenceHash, &item.QueuedBy, &at); err != nil {
			return nil, err
		}
		item.QueuedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, item)
	}
	return out, rows.Err()
}
