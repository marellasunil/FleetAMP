package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

type publicationWorkerStore struct {
	work    *storage.ArtifactPublicationWork
	receipt *distributions.ArtifactPublicationExecution
}

func (s *publicationWorkerStore) Claim(context.Context, string, time.Time, time.Time) (*storage.ArtifactPublicationWork, error) {
	if s.work == nil {
		return nil, storage.ErrArtifactPublicationQueueEmpty
	}
	work := s.work
	s.work = nil
	return work, nil
}
func (s *publicationWorkerStore) Complete(_ context.Context, _ string, receipt *distributions.ArtifactPublicationExecution) error {
	s.receipt = receipt
	return nil
}
func (s *publicationWorkerStore) List(context.Context, int) ([]*distributions.ArtifactPublicationExecution, error) {
	return nil, nil
}
func (s *publicationWorkerStore) AuthorizeRetry(context.Context, *distributions.ArtifactPublicationRetryRequest) error {
	return nil
}
func (s *publicationWorkerStore) ListRetries(context.Context, int) ([]*distributions.ArtifactPublicationRetryRequest, error) {
	return nil, nil
}

type publicationWorkerPublisher struct{ err error }

func (p publicationWorkerPublisher) Publish(_ context.Context, preflight *distributions.ArtifactPublicationPreflight) (*distributions.RegistryPublishResult, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &distributions.RegistryPublishResult{ArtifactSHA256: preflight.ArtifactSHA256, DestinationRef: preflight.DestinationRef}, nil
}

func publicationWorkerFixture() *storage.ArtifactPublicationWork {
	publication := &distributions.ArtifactPublication{ID: "publication-1", ArtifactID: "artifact-1", ArtifactSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", DestinationRef: "oci://registry.example.test/fleetamp:v1", CredentialRef: "secret://registry", Status: distributions.ArtifactPublicationQueued, EvidenceHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}
	preflight := &distributions.ArtifactPublicationPreflight{ID: "preflight-1", PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, ArtifactID: publication.ArtifactID, ArtifactSHA256: publication.ArtifactSHA256, DestinationRef: publication.DestinationRef, CredentialRef: publication.CredentialRef, Status: distributions.ArtifactPublicationPreflightPassed, EvidenceHash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"}
	return &storage.ArtifactPublicationWork{Publication: publication, Preflight: preflight, AttemptNumber: 1, Trigger: distributions.ArtifactPublicationTriggerAutomatic}
}

func TestArtifactPublicationWorkerRecordsSuccess(t *testing.T) {
	store := &publicationWorkerStore{work: publicationWorkerFixture()}
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	worker := &distributionArtifactPublicationWorker{ID: "worker-1", Executions: store, Publisher: publicationWorkerPublisher{}, Now: func() time.Time { return now }}
	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.receipt == nil || store.receipt.Status != distributions.ArtifactPublicationSucceeded {
		t.Fatalf("unexpected receipt: %#v", store.receipt)
	}
}

func TestArtifactPublicationWorkerRecordsSanitizedFailure(t *testing.T) {
	store := &publicationWorkerStore{work: publicationWorkerFixture()}
	worker := &distributionArtifactPublicationWorker{ID: "worker-1", Executions: store, Publisher: publicationWorkerPublisher{err: errors.New("Bearer top-secret")}}
	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.receipt == nil || store.receipt.Status != distributions.ArtifactPublicationFailed || store.receipt.FailureDigest == "" {
		t.Fatalf("unexpected receipt: %#v", store.receipt)
	}
}
