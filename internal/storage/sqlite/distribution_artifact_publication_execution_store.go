package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type DistributionArtifactPublicationExecutionStore struct{ db *sql.DB }

func (s *DistributionArtifactPublicationExecutionStore) Claim(ctx context.Context, worker string, now, until time.Time) (*storage.ArtifactPublicationWork, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO distribution_artifact_publication_jobs(publication_id,preflight_id,status) SELECT publication_id,id,'queued' FROM distribution_artifact_publication_preflights`)
	if err != nil {
		return nil, err
	}
	var publicationID, preflightID string
	err = tx.QueryRowContext(ctx, `SELECT publication_id,preflight_id FROM distribution_artifact_publication_jobs WHERE status='queued' OR (status='running' AND lease_until<=?) ORDER BY rowid LIMIT 1`, formatTime(now)).Scan(&publicationID, &preflightID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, storage.ErrArtifactPublicationQueueEmpty
	}
	if err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE distribution_artifact_publication_jobs SET status='running',worker_id=?,lease_until=?,claimed_at=? WHERE publication_id=? AND (status='queued' OR (status='running' AND lease_until<=?))`, worker, formatTime(until), formatTime(now), publicationID, formatTime(now))
	if err != nil {
		return nil, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return nil, storage.ErrArtifactPublicationQueueEmpty
	}
	publication, err := scanArtifactPublication(tx.QueryRowContext(ctx, `SELECT id,promotion_id,review_id,artifact_id,artifact_sha256,destination_ref,credential_ref,status,evidence_hash,queued_by,queued_at FROM distribution_artifact_publications WHERE id=?`, publicationID))
	if err != nil {
		return nil, err
	}
	preflight, err := scanArtifactPublicationPreflight(tx.QueryRowContext(ctx, `SELECT id,publication_id,publication_evidence_hash,artifact_id,artifact_reference,artifact_sha256,artifact_size,destination_ref,destination_host,credential_ref,credential_available,status,evidence_hash,checked_by,checked_at FROM distribution_artifact_publication_preflights WHERE id=?`, preflightID))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return &storage.ArtifactPublicationWork{Publication: publication, Preflight: preflight}, nil
}

func (s *DistributionArtifactPublicationExecutionStore) Complete(ctx context.Context, worker string, item *distributions.ArtifactPublicationExecution) error {
	if item == nil {
		return fmt.Errorf("artifact publication execution is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var owner, status string
	if err = tx.QueryRowContext(ctx, `SELECT worker_id,status FROM distribution_artifact_publication_jobs WHERE publication_id=?`, item.PublicationID).Scan(&owner, &status); errors.Is(err, sql.ErrNoRows) || owner != worker || status != "running" {
		return storage.ErrArtifactPublicationLeaseConflict
	} else if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO distribution_artifact_publication_executions(id,publication_id,publication_evidence_hash,preflight_id,preflight_evidence_hash,artifact_id,artifact_sha256,destination_ref,credential_ref,status,failure_code,failure_digest,worker_id,evidence_hash,started_at,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.PublicationID, item.PublicationEvidenceHash, item.PreflightID, item.PreflightEvidenceHash, item.ArtifactID, item.ArtifactSHA256, item.DestinationRef, item.CredentialRef, item.Status, item.FailureCode, item.FailureDigest, item.WorkerID, item.EvidenceHash, formatTime(item.StartedAt), formatTime(item.CompletedAt))
	if err != nil {
		return fmt.Errorf("create artifact publication execution: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE distribution_artifact_publication_jobs SET status=?,worker_id='',lease_until='',claimed_at='' WHERE publication_id=? AND worker_id=?`, item.Status, item.PublicationID, worker)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *DistributionArtifactPublicationExecutionStore) List(ctx context.Context, limit int) ([]*distributions.ArtifactPublicationExecution, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,publication_id,publication_evidence_hash,preflight_id,preflight_evidence_hash,artifact_id,artifact_sha256,destination_ref,credential_ref,status,failure_code,failure_digest,worker_id,evidence_hash,started_at,completed_at FROM distribution_artifact_publication_executions ORDER BY completed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*distributions.ArtifactPublicationExecution
	for rows.Next() {
		item := &distributions.ArtifactPublicationExecution{}
		var started, completed string
		if err := rows.Scan(&item.ID, &item.PublicationID, &item.PublicationEvidenceHash, &item.PreflightID, &item.PreflightEvidenceHash, &item.ArtifactID, &item.ArtifactSHA256, &item.DestinationRef, &item.CredentialRef, &item.Status, &item.FailureCode, &item.FailureDigest, &item.WorkerID, &item.EvidenceHash, &started, &completed); err != nil {
			return nil, err
		}
		item.StartedAt, _ = parseTime(started)
		item.CompletedAt, _ = parseTime(completed)
		out = append(out, item)
	}
	return out, rows.Err()
}

type artifactPublicationScanner interface{ Scan(...any) error }

func scanArtifactPublication(scanner artifactPublicationScanner) (*distributions.ArtifactPublication, error) {
	item := &distributions.ArtifactPublication{}
	var at string
	if err := scanner.Scan(&item.ID, &item.PromotionID, &item.ReviewID, &item.ArtifactID, &item.ArtifactSHA256, &item.DestinationRef, &item.CredentialRef, &item.Status, &item.EvidenceHash, &item.QueuedBy, &at); err != nil {
		return nil, err
	}
	item.QueuedAt, _ = time.Parse(time.RFC3339Nano, at)
	return item, nil
}

func scanArtifactPublicationPreflight(scanner artifactPublicationScanner) (*distributions.ArtifactPublicationPreflight, error) {
	item := &distributions.ArtifactPublicationPreflight{}
	var at string
	if err := scanner.Scan(&item.ID, &item.PublicationID, &item.PublicationEvidenceHash, &item.ArtifactID, &item.ArtifactReference, &item.ArtifactSHA256, &item.ArtifactSize, &item.DestinationRef, &item.DestinationHost, &item.CredentialRef, &item.CredentialAvailable, &item.Status, &item.EvidenceHash, &item.CheckedBy, &at); err != nil {
		return nil, err
	}
	item.CheckedAt, _ = time.Parse(time.RFC3339Nano, at)
	return item, nil
}

var _ storage.DistributionArtifactPublicationExecutionStore = (*DistributionArtifactPublicationExecutionStore)(nil)
