package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

func TestArtifactPublicationExecutionClaimAndComplete(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "publication.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// This focused lease test inserts an already-governed publication boundary;
	// the foreign-key chain itself is covered by the individual stores.
	if _, err = db.db.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	sha := strings.Repeat("a", 64)
	if _, err = db.db.ExecContext(ctx, `INSERT INTO distribution_artifact_publications(id,promotion_id,review_id,artifact_id,artifact_sha256,destination_ref,credential_ref,status,evidence_hash,queued_by,queued_at) VALUES('publication-1','promotion-1','review-1','artifact-1',?,'oci://registry.example.test/fleetamp:v1','secret://registry','queued',?,'admin',?)`, sha, strings.Repeat("b", 64), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.db.ExecContext(ctx, `INSERT INTO distribution_artifact_publication_preflights(id,publication_id,publication_evidence_hash,artifact_id,artifact_reference,artifact_sha256,artifact_size,destination_ref,destination_host,credential_ref,credential_available,status,evidence_hash,checked_by,checked_at) VALUES('preflight-1','publication-1',?,'artifact-1',? ,?,8,'oci://registry.example.test/fleetamp:v1','registry.example.test','secret://registry',1,'passed',?,'admin',?)`, strings.Repeat("b", 64), "artifact-cache://"+sha+"/fleetamp-collector", sha, strings.Repeat("c", 64), formatTime(now)); err != nil {
		t.Fatal(err)
	}
	store := db.DistributionArtifactPublicationExecutions()
	work, err := store.Claim(ctx, "worker-1", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if work.Publication.ID != "publication-1" || work.Preflight.ID != "preflight-1" {
		t.Fatalf("unexpected work: %#v", work)
	}
	if _, err = store.Claim(ctx, "worker-2", now, now.Add(time.Minute)); !errors.Is(err, storage.ErrArtifactPublicationQueueEmpty) {
		t.Fatalf("expected active lease to block second claim, got %v", err)
	}
	receipt, err := distributions.NewFailedArtifactPublicationExecution(work.Publication, work.Preflight, distributions.ArtifactPublicationFailurePublisher, "registry unavailable", 1, distributions.ArtifactPublicationTriggerAutomatic, "", "worker-1", now, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, "worker-2", receipt); !errors.Is(err, storage.ErrArtifactPublicationLeaseConflict) {
		t.Fatalf("expected lease conflict, got %v", err)
	}
	if err = store.Complete(ctx, "worker-1", receipt); err != nil {
		t.Fatal(err)
	}
	retry, err := distributions.NewArtifactPublicationRetryRequest(work.Publication, work.Preflight, receipt, "registry recovered", "admin", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AuthorizeRetry(ctx, retry); err != nil {
		t.Fatal(err)
	}
	secondWork, err := store.Claim(ctx, "worker-2", now.Add(3*time.Second), now.Add(4*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if secondWork.AttemptNumber != 2 || secondWork.Trigger != distributions.ArtifactPublicationTriggerManual || secondWork.RetryRequestID != retry.ID {
		t.Fatalf("unexpected retry work: %#v", secondWork)
	}
	result := &distributions.RegistryPublishResult{ArtifactSHA256: sha, DestinationRef: secondWork.Publication.DestinationRef}
	success, err := distributions.NewSuccessfulArtifactPublicationExecution(secondWork.Publication, secondWork.Preflight, result, 2, distributions.ArtifactPublicationTriggerManual, retry.ID, "worker-2", now.Add(3*time.Second), now.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Complete(ctx, "worker-2", success); err != nil {
		t.Fatal(err)
	}
	items, err := store.List(ctx, 10)
	if err != nil || len(items) != 2 || items[0].EvidenceHash != success.EvidenceHash {
		t.Fatalf("unexpected receipts: %#v, %v", items, err)
	}
	if _, err = db.db.ExecContext(ctx, `UPDATE distribution_artifact_publication_executions SET status='failed' WHERE id=?`, success.ID); err == nil {
		t.Fatal("immutable publication receipt accepted update")
	}
	release := &distributions.DistributionRelease{ID: "distribution-release-1", DistributionID: "distribution-1", DistributionSpecHash: strings.Repeat("1", 64), BuildPlanID: "plan-1", BuildPlanHash: strings.Repeat("2", 64), ArtifactID: "artifact-1", ArtifactEvidenceHash: strings.Repeat("3", 64), ArtifactSHA256: sha, PublicationID: work.Publication.ID, PublicationEvidenceHash: work.Publication.EvidenceHash, ExecutionID: success.ID, ExecutionEvidenceHash: success.EvidenceHash, ArtifactURI: work.Publication.DestinationRef, ReleasedBy: "admin", EvidenceHash: strings.Repeat("4", 64), ReleasedAt: now.Add(5 * time.Second)}
	releaseStore := db.DistributionReleases()
	if err = releaseStore.Create(ctx, release); err != nil {
		t.Fatal(err)
	}
	releases, err := releaseStore.List(ctx, 10)
	if err != nil || len(releases) != 1 || releases[0].ExecutionEvidenceHash != success.EvidenceHash {
		t.Fatalf("unexpected releases: %#v, %v", releases, err)
	}
	if _, err = db.db.ExecContext(ctx, `UPDATE distribution_releases SET released_by='other' WHERE id=?`, release.ID); err == nil {
		t.Fatal("immutable distribution release accepted update")
	}
}
