package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestArtifactPublicationExecutionRetryMigrationPreservesReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-publication.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE distribution_artifact_publications (
		id TEXT PRIMARY KEY, promotion_id TEXT NOT NULL UNIQUE, review_id TEXT NOT NULL UNIQUE,
		artifact_id TEXT NOT NULL UNIQUE, artifact_sha256 TEXT NOT NULL, destination_ref TEXT NOT NULL,
		credential_ref TEXT NOT NULL, status TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE,
		queued_by TEXT NOT NULL, queued_at TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE distribution_artifact_publication_preflights (
		id TEXT PRIMARY KEY, publication_id TEXT NOT NULL UNIQUE, publication_evidence_hash TEXT NOT NULL,
		artifact_id TEXT NOT NULL, artifact_reference TEXT NOT NULL, artifact_sha256 TEXT NOT NULL, artifact_size INTEGER NOT NULL,
		destination_ref TEXT NOT NULL, destination_host TEXT NOT NULL, credential_ref TEXT NOT NULL,
		credential_available INTEGER NOT NULL, status TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE,
		checked_by TEXT NOT NULL, checked_at TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`INSERT INTO distribution_artifact_publications VALUES('publication-1','promotion-1','review-1','artifact-1','artifact-hash','oci://registry.example.test/fleetamp:v1','secret://registry','queued','pub-hash','admin','2026-10-22T11:58:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`INSERT INTO distribution_artifact_publication_preflights VALUES('preflight-1','publication-1','pub-hash','artifact-1','artifact-cache://artifact-hash/fleetamp-collector','artifact-hash',8,'oci://registry.example.test/fleetamp:v1','registry.example.test','secret://registry',1,'passed','pre-hash','admin','2026-10-22T11:59:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE distribution_artifact_publication_executions (
		id TEXT PRIMARY KEY, publication_id TEXT NOT NULL UNIQUE, publication_evidence_hash TEXT NOT NULL,
		preflight_id TEXT NOT NULL UNIQUE, preflight_evidence_hash TEXT NOT NULL,
		artifact_id TEXT NOT NULL, artifact_sha256 TEXT NOT NULL, destination_ref TEXT NOT NULL, credential_ref TEXT NOT NULL,
		status TEXT NOT NULL, failure_code TEXT NOT NULL, failure_digest TEXT NOT NULL,
		worker_id TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE, started_at TEXT NOT NULL, completed_at TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`INSERT INTO distribution_artifact_publication_executions VALUES('execution-1','publication-1','pub-hash','preflight-1','pre-hash','artifact-1','artifact-hash','oci://registry.example.test/fleetamp:v1','secret://registry','failed','publisher-error','failure-hash','worker-1','evidence-hash','2026-10-22T12:00:00Z','2026-10-22T12:00:01Z')`)
	if err != nil {
		t.Fatal(err)
	}
	if err = raw.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var attempt int
	var trigger, retryID string
	if err = db.db.QueryRow(`SELECT attempt_number,trigger,retry_request_id FROM distribution_artifact_publication_executions WHERE id='execution-1'`).Scan(&attempt, &trigger, &retryID); err != nil {
		t.Fatal(err)
	}
	if attempt != 1 || trigger != "automatic" || retryID != "" {
		t.Fatalf("legacy receipt was not preserved as attempt 1: %d %q %q", attempt, trigger, retryID)
	}
}
