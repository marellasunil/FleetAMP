// SQLite persistence backend for FleetAMP configuration state.
//
// Purpose:
//
//	Opens the embedded FleetAMP database and owns schema initialization for
//	configuration artifacts, current assignments, and append-only deployment history.
//
// Packaging:
//
//	Uses modernc.org/sqlite, a CGo-free SQLite driver, so FleetAMP can ship as
//	a single Go binary without requiring a system SQLite installation.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type Database struct{ db *sql.DB }

// Open creates the SQLite connection, applies safe connection settings, and initializes or migrates the schema.
func Open(ctx context.Context, path string) (*Database, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create sqlite directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	// SQLite PRAGMAs such as busy_timeout are connection-local. Keep one
	// connection for this embedded write-heavy control-plane database so
	// initialization settings also apply to OpAMP status updates.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	database := &Database{db: db}
	if err := database.initialize(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return database, nil
}

// Close releases the underlying SQLite connection.
func (d *Database) Close() error { return d.db.Close() }

// Configurations returns the SQLite-backed configuration repository.
func (d *Database) Configurations() *ConfigStore { return &ConfigStore{db: d.db} }

// Assignments returns the SQLite-backed desired-state assignment repository.
func (d *Database) Assignments() *AssignmentStore { return &AssignmentStore{db: d.db} }

// Deployments returns the SQLite-backed delivery-history repository.
func (d *Database) Deployments() *DeploymentStore { return &DeploymentStore{db: d.db} }

// Groups returns the SQLite-backed group repository.
func (d *Database) Groups() *GroupStore { return &GroupStore{db: d.db} }

// GroupDeploymentRequests returns the approval-request repository.
func (d *Database) GroupDeploymentRequests() *GroupDeploymentRequestStore {
	return &GroupDeploymentRequestStore{db: d.db}
}

func (d *Database) MembershipChanges() *MembershipChangeStore {
	return &MembershipChangeStore{db: d.db}
}

// DestinationProfiles returns administrator-controlled exporter destinations.
func (d *Database) DestinationProfiles() *DestinationProfileStore {
	return &DestinationProfileStore{db: d.db}
}

// Authentication returns the SQLite-backed user repository.
func (d *Database) Authentication() *AuthStore { return &AuthStore{db: d.db} }

// SectionPolicies returns the SQLite-backed configuration-section policy repository.
func (d *Database) SectionPolicies() *SectionPolicyStore { return &SectionPolicyStore{db: d.db} }

// DriftPolicy returns the SQLite-backed global drift policy repository.
func (d *Database) DriftPolicy() *DriftPolicyStore { return &DriftPolicyStore{db: d.db} }

// Audit returns the SQLite-backed append-only audit repository.
func (d *Database) Audit() *AuditStore { return &AuditStore{db: d.db} }

func (d *Database) GuideCatalog() *GuideCatalogStore { return &GuideCatalogStore{db: d.db} }

func (d *Database) GroupSecrets() *GroupSecretStore { return &GroupSecretStore{db: d.db} }

// Migrations returns the append-only migration provenance repository.
func (d *Database) Migrations() *MigrationStore { return &MigrationStore{db: d.db} }

// ComponentLifecycleRequests returns the immutable component proposal repository.
func (d *Database) ComponentLifecycleRequests() *ComponentLifecycleRequestStore {
	return &ComponentLifecycleRequestStore{db: d.db}
}

// ComponentLifecycleValidations returns immutable target and compatibility snapshots.
func (d *Database) ComponentLifecycleValidations() *ComponentLifecycleValidationStore {
	return &ComponentLifecycleValidationStore{db: d.db}
}

func (d *Database) ComponentLifecycleApprovals() *ComponentLifecycleApprovalStore {
	return &ComponentLifecycleApprovalStore{db: d.db}
}
func (d *Database) ComponentLifecycleExecutions() *ComponentLifecycleExecutionStore {
	return &ComponentLifecycleExecutionStore{db: d.db}
}
func (d *Database) ComponentGitOpsPreviews() *ComponentGitOpsPreviewStore {
	return &ComponentGitOpsPreviewStore{db: d.db}
}
func (d *Database) GitOpsPreviewApprovals() *GitOpsPreviewApprovalStore {
	return &GitOpsPreviewApprovalStore{db: d.db}
}
func (d *Database) GitOpsExecutions() *GitOpsExecutionStore {
	return &GitOpsExecutionStore{db: d.db}
}
func (d *Database) IntegrationConnections() *IntegrationConnectionStore {
	return &IntegrationConnectionStore{db: d.db}
}
func (d *Database) IntegrationValidations() *IntegrationValidationStore {
	return &IntegrationValidationStore{db: d.db}
}
func (d *Database) Distributions() *DistributionStore { return &DistributionStore{db: d.db} }
func (d *Database) DistributionPreviews() *DistributionPreviewStore {
	return &DistributionPreviewStore{db: d.db}
}
func (d *Database) DistributionBuildPlans() *DistributionBuildPlanStore {
	return &DistributionBuildPlanStore{db: d.db}
}
func (d *Database) DistributionBuildApprovals() *DistributionBuildApprovalStore {
	return &DistributionBuildApprovalStore{db: d.db}
}
func (d *Database) DistributionBuildExecutions() *DistributionBuildExecutionStore {
	return &DistributionBuildExecutionStore{db: d.db}
}
func (d *Database) DistributionBuildPreflights() *DistributionBuildPreflightStore {
	return &DistributionBuildPreflightStore{db: d.db}
}
func (d *Database) DistributionBuildWorkerResults() *DistributionBuildWorkerResultStore {
	return &DistributionBuildWorkerResultStore{db: d.db}
}
func (d *Database) DistributionBuildSources() *DistributionBuildSourceStore {
	return &DistributionBuildSourceStore{db: d.db}
}
func (d *Database) DistributionBuildSourceAcquisitions() *DistributionBuildSourceAcquisitionStore {
	return &DistributionBuildSourceAcquisitionStore{db: d.db}
}
func (d *Database) DistributionBuildSourceInspections() *DistributionBuildSourceInspectionStore {
	return &DistributionBuildSourceInspectionStore{db: d.db}
}
func (d *Database) DistributionBuildArtifacts() *DistributionBuildArtifactStore {
	return &DistributionBuildArtifactStore{db: d.db}
}
func (d *Database) DistributionBuildAttempts() *DistributionBuildAttemptStore {
	return &DistributionBuildAttemptStore{db: d.db}
}
func (d *Database) DistributionBuildRetryRequests() *DistributionBuildRetryRequestStore {
	return &DistributionBuildRetryRequestStore{db: d.db}
}
func (d *Database) DistributionArtifactPromotions() *DistributionArtifactPromotionStore {
	return &DistributionArtifactPromotionStore{db: d.db}
}
func (d *Database) DistributionArtifactPromotionReviews() *DistributionArtifactPromotionReviewStore {
	return &DistributionArtifactPromotionReviewStore{db: d.db}
}
func (d *Database) DistributionArtifactPublications() *DistributionArtifactPublicationStore {
	return &DistributionArtifactPublicationStore{db: d.db}
}
func (d *Database) DistributionArtifactPublicationPreflights() *DistributionArtifactPublicationPreflightStore {
	return &DistributionArtifactPublicationPreflightStore{db: d.db}
}
func (d *Database) DistributionArtifactPublicationExecutions() *DistributionArtifactPublicationExecutionStore {
	return &DistributionArtifactPublicationExecutionStore{db: d.db}
}
func (d *Database) DistributionReleases() *DistributionReleaseStore {
	return &DistributionReleaseStore{db: d.db}
}

// initialize creates all required tables and indexes in an idempotent transaction.
func (d *Database) initialize(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS collector_distributions (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, collector_version TEXT NOT NULL,
			source_repository TEXT NOT NULL, source_ref TEXT NOT NULL,
			artifact_uri TEXT NOT NULL DEFAULT '', artifact_sha256 TEXT NOT NULL DEFAULT '',
			components TEXT NOT NULL, locked_sections TEXT NOT NULL,
			editable_sections TEXT NOT NULL, baseline_config TEXT NOT NULL DEFAULT '', spec_hash TEXT NOT NULL UNIQUE,
			published_by TEXT NOT NULL, published_at TEXT NOT NULL,
			UNIQUE(name,collector_version)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_collector_distributions_published ON collector_distributions(published_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS collector_distributions_no_update
			BEFORE UPDATE ON collector_distributions BEGIN SELECT RAISE(ABORT,'Collector distributions are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS collector_distributions_no_delete
			BEFORE DELETE ON collector_distributions BEGIN SELECT RAISE(ABORT,'Collector distributions are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_plans (
			id TEXT PRIMARY KEY, distribution_id TEXT NOT NULL, manifest TEXT NOT NULL,
			plan_hash TEXT NOT NULL UNIQUE, status TEXT NOT NULL,
			requested_by TEXT NOT NULL, requested_at TEXT NOT NULL,
			FOREIGN KEY(distribution_id) REFERENCES collector_distributions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_plans_requested ON distribution_build_plans(requested_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_plans_no_update BEFORE UPDATE ON distribution_build_plans BEGIN SELECT RAISE(ABORT,'Distribution build plans are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_plans_no_delete BEFORE DELETE ON distribution_build_plans BEGIN SELECT RAISE(ABORT,'Distribution build plans are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_approvals (
			id TEXT PRIMARY KEY, build_plan_id TEXT NOT NULL UNIQUE, build_plan_hash TEXT NOT NULL,
			decision TEXT NOT NULL CHECK(decision IN ('approved','rejected')), reason TEXT NOT NULL,
			reviewed_by TEXT NOT NULL, reviewed_at TEXT NOT NULL,
			FOREIGN KEY(build_plan_id) REFERENCES distribution_build_plans(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_approvals_reviewed ON distribution_build_approvals(reviewed_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_approvals_no_update BEFORE UPDATE ON distribution_build_approvals BEGIN SELECT RAISE(ABORT,'Distribution build approvals are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_approvals_no_delete BEFORE DELETE ON distribution_build_approvals BEGIN SELECT RAISE(ABORT,'Distribution build approvals are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_executions (
			id TEXT PRIMARY KEY, build_plan_id TEXT NOT NULL UNIQUE, build_plan_hash TEXT NOT NULL,
			approval_id TEXT NOT NULL UNIQUE, approval_reviewed_by TEXT NOT NULL, approval_reviewed_at TEXT NOT NULL,
			manifest TEXT NOT NULL, executor TEXT NOT NULL CHECK(executor = 'disabled'),
			status TEXT NOT NULL CHECK(status = 'queued'), queued_by TEXT NOT NULL, queued_at TEXT NOT NULL,
			FOREIGN KEY(build_plan_id) REFERENCES distribution_build_plans(id),
			FOREIGN KEY(approval_id) REFERENCES distribution_build_approvals(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_executions_queued ON distribution_build_executions(queued_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_executions_no_update BEFORE UPDATE ON distribution_build_executions BEGIN SELECT RAISE(ABORT,'Distribution build executions are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_executions_no_delete BEFORE DELETE ON distribution_build_executions BEGIN SELECT RAISE(ABORT,'Distribution build executions are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_preflights (
			id TEXT PRIMARY KEY, execution_id TEXT NOT NULL UNIQUE, build_plan_id TEXT NOT NULL,
			build_plan_hash TEXT NOT NULL, manifest_hash TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status = 'ready-for-worker'), checked_by TEXT NOT NULL, checked_at TEXT NOT NULL,
			FOREIGN KEY(execution_id) REFERENCES distribution_build_executions(id),
			FOREIGN KEY(build_plan_id) REFERENCES distribution_build_plans(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_preflights_checked ON distribution_build_preflights(checked_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_preflights_no_update BEFORE UPDATE ON distribution_build_preflights BEGIN SELECT RAISE(ABORT,'Distribution build preflights are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_preflights_no_delete BEFORE DELETE ON distribution_build_preflights BEGIN SELECT RAISE(ABORT,'Distribution build preflights are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_worker_results (
			id TEXT PRIMARY KEY, execution_id TEXT NOT NULL UNIQUE, preflight_id TEXT NOT NULL UNIQUE,
			build_plan_hash TEXT NOT NULL, manifest_hash TEXT NOT NULL, worker_id TEXT NOT NULL,
			adapter TEXT NOT NULL CHECK(adapter = 'dry-run'), status TEXT NOT NULL CHECK(status = 'dry-run-complete'),
			evidence_hash TEXT NOT NULL UNIQUE, completed_at TEXT NOT NULL,
			FOREIGN KEY(execution_id) REFERENCES distribution_build_executions(id),
			FOREIGN KEY(preflight_id) REFERENCES distribution_build_preflights(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_worker_results_completed ON distribution_build_worker_results(completed_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_worker_results_no_update BEFORE UPDATE ON distribution_build_worker_results BEGIN SELECT RAISE(ABORT,'Distribution build worker results are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_worker_results_no_delete BEFORE DELETE ON distribution_build_worker_results BEGIN SELECT RAISE(ABORT,'Distribution build worker results are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_sources (
			id TEXT PRIMARY KEY, execution_id TEXT NOT NULL UNIQUE, worker_result_id TEXT NOT NULL UNIQUE,
			build_plan_hash TEXT NOT NULL, repository TEXT NOT NULL, source_ref TEXT NOT NULL,
			archive_url TEXT NOT NULL, status TEXT NOT NULL CHECK(status = 'source-resolved'),
			evidence_hash TEXT NOT NULL UNIQUE, resolved_by TEXT NOT NULL, resolved_at TEXT NOT NULL,
			FOREIGN KEY(execution_id) REFERENCES distribution_build_executions(id),
			FOREIGN KEY(worker_result_id) REFERENCES distribution_build_worker_results(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_sources_resolved ON distribution_build_sources(resolved_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_sources_no_update BEFORE UPDATE ON distribution_build_sources BEGIN SELECT RAISE(ABORT,'Distribution build sources are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_sources_no_delete BEFORE DELETE ON distribution_build_sources BEGIN SELECT RAISE(ABORT,'Distribution build sources are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_source_acquisitions (
			id TEXT PRIMARY KEY, source_resolution_id TEXT NOT NULL UNIQUE, execution_id TEXT NOT NULL UNIQUE,
			build_plan_hash TEXT NOT NULL, archive_sha256 TEXT NOT NULL, archive_size INTEGER NOT NULL CHECK(archive_size > 0),
			local_reference TEXT NOT NULL, status TEXT NOT NULL CHECK(status = 'source-acquired'),
			evidence_hash TEXT NOT NULL UNIQUE, acquired_by TEXT NOT NULL, acquired_at TEXT NOT NULL,
			FOREIGN KEY(source_resolution_id) REFERENCES distribution_build_sources(id),
			FOREIGN KEY(execution_id) REFERENCES distribution_build_executions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_source_acquisitions_time ON distribution_build_source_acquisitions(acquired_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_source_acquisitions_no_update BEFORE UPDATE ON distribution_build_source_acquisitions BEGIN SELECT RAISE(ABORT,'Distribution source acquisitions are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_source_acquisitions_no_delete BEFORE DELETE ON distribution_build_source_acquisitions BEGIN SELECT RAISE(ABORT,'Distribution source acquisitions are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_source_inspections (
			id TEXT PRIMARY KEY, acquisition_id TEXT NOT NULL UNIQUE, execution_id TEXT NOT NULL UNIQUE,
			build_plan_hash TEXT NOT NULL, archive_sha256 TEXT NOT NULL,
			entry_count INTEGER NOT NULL, file_count INTEGER NOT NULL, directory_count INTEGER NOT NULL,
			uncompressed_bytes INTEGER NOT NULL, root_prefix TEXT NOT NULL,
			manifest_yaml TEXT NOT NULL, manifest_sha256 TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL CHECK(status = 'source-inspected'), evidence_hash TEXT NOT NULL UNIQUE,
			inspected_by TEXT NOT NULL, inspected_at TEXT NOT NULL,
			FOREIGN KEY(acquisition_id) REFERENCES distribution_build_source_acquisitions(id),
			FOREIGN KEY(execution_id) REFERENCES distribution_build_executions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_source_inspections_time ON distribution_build_source_inspections(inspected_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_source_inspections_no_update BEFORE UPDATE ON distribution_build_source_inspections BEGIN SELECT RAISE(ABORT,'Distribution source inspections are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_source_inspections_no_delete BEFORE DELETE ON distribution_build_source_inspections BEGIN SELECT RAISE(ABORT,'Distribution source inspections are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_artifacts (
			id TEXT PRIMARY KEY, inspection_id TEXT NOT NULL UNIQUE, execution_id TEXT NOT NULL UNIQUE,
			build_plan_hash TEXT NOT NULL, manifest_sha256 TEXT NOT NULL,
			runtime TEXT NOT NULL CHECK(runtime IN ('docker','podman')), builder_image TEXT NOT NULL,
			artifact_reference TEXT NOT NULL, artifact_sha256 TEXT NOT NULL,
			artifact_size INTEGER NOT NULL CHECK(artifact_size > 0),
			status TEXT NOT NULL CHECK(status = 'artifact-generated'), evidence_hash TEXT NOT NULL UNIQUE,
			built_by TEXT NOT NULL, built_at TEXT NOT NULL,
			FOREIGN KEY(inspection_id) REFERENCES distribution_build_source_inspections(id),
			FOREIGN KEY(execution_id) REFERENCES distribution_build_executions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_artifacts_time ON distribution_build_artifacts(built_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_artifacts_no_update BEFORE UPDATE ON distribution_build_artifacts BEGIN SELECT RAISE(ABORT,'Distribution build artifacts are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_artifacts_no_delete BEFORE DELETE ON distribution_build_artifacts BEGIN SELECT RAISE(ABORT,'Distribution build artifacts are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_attempts (
			id TEXT PRIMARY KEY, inspection_id TEXT NOT NULL, execution_id TEXT NOT NULL,
			build_plan_hash TEXT NOT NULL, manifest_sha256 TEXT NOT NULL,
			attempt_number INTEGER NOT NULL CHECK(attempt_number > 0),
			trigger TEXT NOT NULL CHECK(trigger IN ('automatic','manual')), retry_request_id TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK(status IN ('succeeded','failed')),
			failure_code TEXT NOT NULL DEFAULT '', failure_message TEXT NOT NULL DEFAULT '', failure_digest TEXT NOT NULL DEFAULT '',
			artifact_id TEXT NOT NULL DEFAULT '', artifact_sha256 TEXT NOT NULL DEFAULT '',
			worker_id TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE,
			started_at TEXT NOT NULL, completed_at TEXT NOT NULL, next_retry_at TEXT NOT NULL DEFAULT '',
			UNIQUE(inspection_id,attempt_number),
			FOREIGN KEY(inspection_id) REFERENCES distribution_build_source_inspections(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_attempts_time ON distribution_build_attempts(completed_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_attempts_no_update BEFORE UPDATE ON distribution_build_attempts BEGIN SELECT RAISE(ABORT,'Distribution build attempts are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_attempts_no_delete BEFORE DELETE ON distribution_build_attempts BEGIN SELECT RAISE(ABORT,'Distribution build attempts are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_build_retry_requests (
			id TEXT PRIMARY KEY, inspection_id TEXT NOT NULL, previous_attempt_id TEXT NOT NULL UNIQUE,
			reason TEXT NOT NULL, requested_by TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE, requested_at TEXT NOT NULL,
			FOREIGN KEY(inspection_id) REFERENCES distribution_build_source_inspections(id),
			FOREIGN KEY(previous_attempt_id) REFERENCES distribution_build_attempts(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_build_retry_requests_time ON distribution_build_retry_requests(requested_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_retry_requests_no_update BEFORE UPDATE ON distribution_build_retry_requests BEGIN SELECT RAISE(ABORT,'Distribution build retry requests are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_build_retry_requests_no_delete BEFORE DELETE ON distribution_build_retry_requests BEGIN SELECT RAISE(ABORT,'Distribution build retry requests are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_artifact_promotions (
			id TEXT PRIMARY KEY, artifact_id TEXT NOT NULL UNIQUE, artifact_sha256 TEXT NOT NULL,
			build_plan_hash TEXT NOT NULL, destination_ref TEXT NOT NULL, credential_ref TEXT NOT NULL,
			reason TEXT NOT NULL, requested_by TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status = 'pending'), evidence_hash TEXT NOT NULL UNIQUE, requested_at TEXT NOT NULL,
			FOREIGN KEY(artifact_id) REFERENCES distribution_build_artifacts(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_artifact_promotions_time ON distribution_artifact_promotions(requested_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_promotions_no_update BEFORE UPDATE ON distribution_artifact_promotions BEGIN SELECT RAISE(ABORT,'Distribution artifact promotions are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_promotions_no_delete BEFORE DELETE ON distribution_artifact_promotions BEGIN SELECT RAISE(ABORT,'Distribution artifact promotions are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_artifact_promotion_reviews (
			id TEXT PRIMARY KEY, promotion_id TEXT NOT NULL UNIQUE, promotion_evidence_hash TEXT NOT NULL,
			artifact_sha256 TEXT NOT NULL, decision TEXT NOT NULL CHECK(decision IN ('approved','rejected')),
			reason TEXT NOT NULL, reviewed_by TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE, reviewed_at TEXT NOT NULL,
			FOREIGN KEY(promotion_id) REFERENCES distribution_artifact_promotions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_artifact_promotion_reviews_time ON distribution_artifact_promotion_reviews(reviewed_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_promotion_reviews_no_update BEFORE UPDATE ON distribution_artifact_promotion_reviews BEGIN SELECT RAISE(ABORT,'Distribution artifact promotion reviews are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_promotion_reviews_no_delete BEFORE DELETE ON distribution_artifact_promotion_reviews BEGIN SELECT RAISE(ABORT,'Distribution artifact promotion reviews are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_artifact_publications (
			id TEXT PRIMARY KEY, promotion_id TEXT NOT NULL UNIQUE, review_id TEXT NOT NULL UNIQUE,
			artifact_id TEXT NOT NULL UNIQUE, artifact_sha256 TEXT NOT NULL,
			destination_ref TEXT NOT NULL, credential_ref TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status = 'queued'), evidence_hash TEXT NOT NULL UNIQUE,
			queued_by TEXT NOT NULL, queued_at TEXT NOT NULL,
			FOREIGN KEY(promotion_id) REFERENCES distribution_artifact_promotions(id),
			FOREIGN KEY(review_id) REFERENCES distribution_artifact_promotion_reviews(id),
			FOREIGN KEY(artifact_id) REFERENCES distribution_build_artifacts(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_artifact_publications_time ON distribution_artifact_publications(queued_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publications_no_update BEFORE UPDATE ON distribution_artifact_publications BEGIN SELECT RAISE(ABORT,'Distribution artifact publications are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publications_no_delete BEFORE DELETE ON distribution_artifact_publications BEGIN SELECT RAISE(ABORT,'Distribution artifact publications are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_artifact_publication_preflights (
			id TEXT PRIMARY KEY, publication_id TEXT NOT NULL UNIQUE, publication_evidence_hash TEXT NOT NULL,
			artifact_id TEXT NOT NULL, artifact_reference TEXT NOT NULL, artifact_sha256 TEXT NOT NULL, artifact_size INTEGER NOT NULL,
			destination_ref TEXT NOT NULL, destination_host TEXT NOT NULL, credential_ref TEXT NOT NULL,
			credential_available INTEGER NOT NULL CHECK(credential_available = 1), status TEXT NOT NULL CHECK(status = 'passed'),
			evidence_hash TEXT NOT NULL UNIQUE, checked_by TEXT NOT NULL, checked_at TEXT NOT NULL,
			FOREIGN KEY(publication_id) REFERENCES distribution_artifact_publications(id),
			FOREIGN KEY(artifact_id) REFERENCES distribution_build_artifacts(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_artifact_publication_preflights_time ON distribution_artifact_publication_preflights(checked_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publication_preflights_no_update BEFORE UPDATE ON distribution_artifact_publication_preflights BEGIN SELECT RAISE(ABORT,'Distribution artifact publication preflights are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publication_preflights_no_delete BEFORE DELETE ON distribution_artifact_publication_preflights BEGIN SELECT RAISE(ABORT,'Distribution artifact publication preflights are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_artifact_publication_jobs (
			publication_id TEXT PRIMARY KEY, preflight_id TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL CHECK(status IN ('queued','running','succeeded','failed')),
			worker_id TEXT NOT NULL DEFAULT '', lease_until TEXT NOT NULL DEFAULT '', claimed_at TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(publication_id) REFERENCES distribution_artifact_publications(id),
			FOREIGN KEY(preflight_id) REFERENCES distribution_artifact_publication_preflights(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_artifact_publication_jobs_status ON distribution_artifact_publication_jobs(status,lease_until)`,
		`CREATE TABLE IF NOT EXISTS distribution_artifact_publication_executions (
			id TEXT PRIMARY KEY, publication_id TEXT NOT NULL, publication_evidence_hash TEXT NOT NULL,
			preflight_id TEXT NOT NULL, preflight_evidence_hash TEXT NOT NULL,
			artifact_id TEXT NOT NULL, artifact_sha256 TEXT NOT NULL, destination_ref TEXT NOT NULL, credential_ref TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('succeeded','failed')), failure_code TEXT NOT NULL, failure_digest TEXT NOT NULL,
			attempt_number INTEGER NOT NULL, trigger TEXT NOT NULL CHECK(trigger IN ('automatic','manual')), retry_request_id TEXT NOT NULL,
			worker_id TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE, started_at TEXT NOT NULL, completed_at TEXT NOT NULL,
			UNIQUE(publication_id,attempt_number),
			FOREIGN KEY(publication_id) REFERENCES distribution_artifact_publications(id),
			FOREIGN KEY(preflight_id) REFERENCES distribution_artifact_publication_preflights(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_artifact_publication_executions_time ON distribution_artifact_publication_executions(completed_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publication_executions_no_update BEFORE UPDATE ON distribution_artifact_publication_executions BEGIN SELECT RAISE(ABORT,'Distribution artifact publication executions are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publication_executions_no_delete BEFORE DELETE ON distribution_artifact_publication_executions BEGIN SELECT RAISE(ABORT,'Distribution artifact publication executions are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_artifact_publication_retry_requests (
			id TEXT PRIMARY KEY, publication_id TEXT NOT NULL, publication_evidence_hash TEXT NOT NULL,
			preflight_id TEXT NOT NULL, preflight_evidence_hash TEXT NOT NULL,
			previous_execution_id TEXT NOT NULL UNIQUE, previous_execution_evidence_hash TEXT NOT NULL,
			reason TEXT NOT NULL, requested_by TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE, requested_at TEXT NOT NULL,
			FOREIGN KEY(publication_id) REFERENCES distribution_artifact_publications(id),
			FOREIGN KEY(preflight_id) REFERENCES distribution_artifact_publication_preflights(id),
			FOREIGN KEY(previous_execution_id) REFERENCES distribution_artifact_publication_executions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_artifact_publication_retries_time ON distribution_artifact_publication_retry_requests(requested_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publication_retry_requests_no_update BEFORE UPDATE ON distribution_artifact_publication_retry_requests BEGIN SELECT RAISE(ABORT,'Distribution artifact publication retry requests are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_artifact_publication_retry_requests_no_delete BEFORE DELETE ON distribution_artifact_publication_retry_requests BEGIN SELECT RAISE(ABORT,'Distribution artifact publication retry requests are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_releases (
			id TEXT PRIMARY KEY, distribution_id TEXT NOT NULL UNIQUE, distribution_spec_hash TEXT NOT NULL,
			build_plan_id TEXT NOT NULL, build_plan_hash TEXT NOT NULL, artifact_id TEXT NOT NULL UNIQUE,
			artifact_evidence_hash TEXT NOT NULL, artifact_sha256 TEXT NOT NULL,
			publication_id TEXT NOT NULL UNIQUE, publication_evidence_hash TEXT NOT NULL,
			execution_id TEXT NOT NULL UNIQUE, execution_evidence_hash TEXT NOT NULL,
			artifact_uri TEXT NOT NULL, released_by TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE, released_at TEXT NOT NULL,
			FOREIGN KEY(distribution_id) REFERENCES collector_distributions(id),
			FOREIGN KEY(build_plan_id) REFERENCES distribution_build_plans(id),
			FOREIGN KEY(artifact_id) REFERENCES distribution_build_artifacts(id),
			FOREIGN KEY(publication_id) REFERENCES distribution_artifact_publications(id),
			FOREIGN KEY(execution_id) REFERENCES distribution_artifact_publication_executions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_releases_time ON distribution_releases(released_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_releases_no_update BEFORE UPDATE ON distribution_releases BEGIN SELECT RAISE(ABORT,'Distribution releases are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_releases_no_delete BEFORE DELETE ON distribution_releases BEGIN SELECT RAISE(ABORT,'Distribution releases are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS distribution_deployment_previews (
			id TEXT PRIMARY KEY, distribution_id TEXT NOT NULL, distribution_spec_hash TEXT NOT NULL,
			release_id TEXT NOT NULL, release_evidence_hash TEXT NOT NULL, artifact_uri TEXT NOT NULL, artifact_sha256 TEXT NOT NULL,
			group_id TEXT NOT NULL, group_name TEXT NOT NULL, sections TEXT NOT NULL,
			composed_config TEXT NOT NULL, content_hash TEXT NOT NULL,
			validation_evidence TEXT NOT NULL, prepared_by TEXT NOT NULL, created_at TEXT NOT NULL,
			FOREIGN KEY(distribution_id) REFERENCES collector_distributions(id),
			FOREIGN KEY(group_id) REFERENCES groups(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_distribution_previews_created ON distribution_deployment_previews(created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS distribution_previews_no_update BEFORE UPDATE ON distribution_deployment_previews BEGIN SELECT RAISE(ABORT,'distribution previews are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS distribution_previews_no_delete BEFORE DELETE ON distribution_deployment_previews BEGIN SELECT RAISE(ABORT,'distribution previews are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS configurations (
			id TEXT PRIMARY KEY, group_id TEXT NOT NULL DEFAULT '', name TEXT NOT NULL, version TEXT NOT NULL,
            content TEXT NOT NULL, content_type TEXT NOT NULL, hash TEXT NOT NULL,
            created_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_configurations_created_at ON configurations(created_at)`,
		`CREATE TABLE IF NOT EXISTS migration_history (
			id TEXT PRIMARY KEY, configuration_id TEXT NOT NULL UNIQUE,
			group_id TEXT NOT NULL, group_name TEXT NOT NULL, name TEXT NOT NULL, version TEXT NOT NULL,
			source TEXT NOT NULL, agent_uid TEXT NOT NULL DEFAULT '', pattern_id TEXT NOT NULL,
			content_hash TEXT NOT NULL, created_by TEXT NOT NULL, created_at TEXT NOT NULL,
			FOREIGN KEY(configuration_id) REFERENCES configurations(id),
			FOREIGN KEY(group_id) REFERENCES groups(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_migration_history_group_created ON migration_history(group_id,created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS assignments (
            agent_instance_uid TEXT NOT NULL, configuration_id TEXT NOT NULL,
            configuration_hash TEXT NOT NULL, status TEXT NOT NULL,
            error TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL,
            PRIMARY KEY(agent_instance_uid, configuration_id),
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_agent_hash ON assignments(agent_instance_uid, configuration_hash)`,
		`CREATE INDEX IF NOT EXISTS idx_assignments_updated_at ON assignments(updated_at)`,
		`CREATE TABLE IF NOT EXISTS deployments (
            id TEXT PRIMARY KEY, agent_instance_uid TEXT NOT NULL,
            configuration_id TEXT NOT NULL, configuration_name TEXT NOT NULL,
            configuration_version TEXT NOT NULL, configuration_hash TEXT NOT NULL,
            previous_configuration_id TEXT NOT NULL DEFAULT '', approval_request_id TEXT NOT NULL DEFAULT '',
            rollback_started_at TEXT,
            action TEXT NOT NULL, status TEXT NOT NULL, error TEXT NOT NULL DEFAULT '',
            created_at TEXT NOT NULL, sent_at TEXT, applying_at TEXT, applied_at TEXT,
            failed_at TEXT, updated_at TEXT NOT NULL,
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_agent_created ON deployments(agent_instance_uid, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_deployments_agent_hash ON deployments(agent_instance_uid, configuration_hash, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS groups (
            id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '',
            selector TEXT NOT NULL, membership_rule TEXT NOT NULL DEFAULT 'null', owners TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name)`,
		`CREATE TABLE IF NOT EXISTS group_secrets (
			group_id TEXT NOT NULL, secret_key TEXT NOT NULL, ciphertext TEXT NOT NULL,
			updated_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			PRIMARY KEY(group_id,secret_key), FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS destination_profiles (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, environment TEXT NOT NULL,
			exporter_id TEXT NOT NULL, exporter_config TEXT NOT NULL,
			owner TEXT NOT NULL DEFAULT 'Platform Team',
			visibility TEXT NOT NULL DEFAULT 'organization',
			group_ids TEXT NOT NULL DEFAULT '[]',
			enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(name,environment)
		)`,
		`CREATE TABLE IF NOT EXISTS blueprint_patterns (
			id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, description TEXT NOT NULL DEFAULT '',
			platform TEXT NOT NULL, receiver_id TEXT NOT NULL, receiver_config TEXT NOT NULL,
			signals TEXT NOT NULL DEFAULT '[]', enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS blueprint_blocks (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL, component_id TEXT NOT NULL, config_yaml TEXT NOT NULL,
			signals TEXT NOT NULL DEFAULT '[]', platforms TEXT NOT NULL DEFAULT '[]',
			required INTEGER NOT NULL DEFAULT 0, locked INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(kind,component_id)
		)`,
		`CREATE TABLE IF NOT EXISTS group_deployment_requests (
            id TEXT PRIMARY KEY, group_id TEXT NOT NULL, group_name TEXT NOT NULL,
            group_selector TEXT NOT NULL, label_selector TEXT NOT NULL DEFAULT '{}', configuration_id TEXT NOT NULL,
            configuration_name TEXT NOT NULL, configuration_version TEXT NOT NULL,
            configuration_hash TEXT NOT NULL, base_configuration_id TEXT NOT NULL DEFAULT '',
            base_configuration_hash TEXT NOT NULL DEFAULT '', targets TEXT NOT NULL,
			requested_by TEXT NOT NULL, assigned_reviewer TEXT NOT NULL DEFAULT '',
			change_reason TEXT NOT NULL DEFAULT '', reviewed_by TEXT NOT NULL DEFAULT '',
            review_comment TEXT NOT NULL DEFAULT '', reviewed_at TEXT,
            expires_at TEXT NOT NULL, expired_at TEXT,
            status TEXT NOT NULL, created_at TEXT NOT NULL,
            FOREIGN KEY(group_id) REFERENCES groups(id),
            FOREIGN KEY(configuration_id) REFERENCES configurations(id)
        )`,
		`CREATE INDEX IF NOT EXISTS idx_group_deployment_requests_group_created ON group_deployment_requests(group_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS membership_changes (
			id TEXT PRIMARY KEY, operation TEXT NOT NULL,
			source_group_id TEXT NOT NULL DEFAULT '', source_group_name TEXT NOT NULL DEFAULT '',
			target_group_id TEXT NOT NULL DEFAULT '', target_group_name TEXT NOT NULL DEFAULT '',
			removal_policy TEXT NOT NULL DEFAULT '', target_configuration_id TEXT NOT NULL DEFAULT '',
			targets TEXT NOT NULL, requested_by TEXT NOT NULL, assigned_reviewer TEXT NOT NULL,
			reason TEXT NOT NULL, status TEXT NOT NULL, reviewed_by TEXT NOT NULL DEFAULT '',
			review_comment TEXT NOT NULL DEFAULT '', reviewed_at TEXT, error TEXT NOT NULL DEFAULT '',
			execution_results TEXT NOT NULL DEFAULT '[]', created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_membership_changes_created ON membership_changes(created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_requests (
			id TEXT PRIMARY KEY, operation TEXT NOT NULL, component_type TEXT NOT NULL,
			group_id TEXT NOT NULL, label_selector TEXT NOT NULL DEFAULT '',
			deployment_method TEXT NOT NULL, current_version TEXT NOT NULL DEFAULT '',
			desired_version TEXT NOT NULL DEFAULT '', reason TEXT NOT NULL,
			spec_hash TEXT NOT NULL, requested_by TEXT NOT NULL,
			status TEXT NOT NULL, created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_created ON component_lifecycle_requests(created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_group ON component_lifecycle_requests(group_id,created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_requests_no_update
			BEFORE UPDATE ON component_lifecycle_requests
			BEGIN SELECT RAISE(ABORT,'component lifecycle requests are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_requests_no_delete
			BEFORE DELETE ON component_lifecycle_requests
			BEGIN SELECT RAISE(ABORT,'component lifecycle requests are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_validations (
			id TEXT PRIMARY KEY, request_id TEXT NOT NULL, request_spec_hash TEXT NOT NULL,
			group_id TEXT NOT NULL, group_name TEXT NOT NULL, group_selector TEXT NOT NULL,
			label_selector TEXT NOT NULL, targets TEXT NOT NULL, findings TEXT NOT NULL,
			status TEXT NOT NULL, result_hash TEXT NOT NULL, validated_by TEXT NOT NULL, created_at TEXT NOT NULL,
			FOREIGN KEY(request_id) REFERENCES component_lifecycle_requests(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_validations_request ON component_lifecycle_validations(request_id,created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_validations_no_update
			BEFORE UPDATE ON component_lifecycle_validations
			BEGIN SELECT RAISE(ABORT,'component lifecycle validations are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_validations_no_delete
			BEFORE DELETE ON component_lifecycle_validations
			BEGIN SELECT RAISE(ABORT,'component lifecycle validations are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_approvals (
			id TEXT PRIMARY KEY, request_id TEXT NOT NULL, request_spec_hash TEXT NOT NULL,
			validation_id TEXT NOT NULL UNIQUE, validation_hash TEXT NOT NULL, group_id TEXT NOT NULL, group_name TEXT NOT NULL,
			operation TEXT NOT NULL, component_type TEXT NOT NULL, target_count INTEGER NOT NULL,
			requested_by TEXT NOT NULL, assigned_reviewer TEXT NOT NULL, submission_comment TEXT NOT NULL,
			status TEXT NOT NULL, reviewed_by TEXT NOT NULL DEFAULT '', review_comment TEXT NOT NULL DEFAULT '', reviewed_at TEXT, created_at TEXT NOT NULL,
			FOREIGN KEY(request_id) REFERENCES component_lifecycle_requests(id), FOREIGN KEY(validation_id) REFERENCES component_lifecycle_validations(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_approvals_status ON component_lifecycle_approvals(status,created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS component_lifecycle_execution_plans(
			id TEXT PRIMARY KEY,approval_id TEXT NOT NULL UNIQUE,request_id TEXT NOT NULL,request_spec_hash TEXT NOT NULL,
			validation_id TEXT NOT NULL,validation_hash TEXT NOT NULL,executor_kind TEXT NOT NULL,operation TEXT NOT NULL,
			component_type TEXT NOT NULL,deployment_method TEXT NOT NULL,targets TEXT NOT NULL,plan_hash TEXT NOT NULL,
			prepared_by TEXT NOT NULL,created_at TEXT NOT NULL,
			FOREIGN KEY(approval_id) REFERENCES component_lifecycle_approvals(id),FOREIGN KEY(request_id) REFERENCES component_lifecycle_requests(id),FOREIGN KEY(validation_id) REFERENCES component_lifecycle_validations(id))`,
		`CREATE INDEX IF NOT EXISTS idx_component_lifecycle_execution_created ON component_lifecycle_execution_plans(created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_execution_no_update BEFORE UPDATE ON component_lifecycle_execution_plans BEGIN SELECT RAISE(ABORT,'component lifecycle execution plans are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS component_lifecycle_execution_no_delete BEFORE DELETE ON component_lifecycle_execution_plans BEGIN SELECT RAISE(ABORT,'component lifecycle execution plans are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS component_gitops_previews(id TEXT PRIMARY KEY,plan_id TEXT NOT NULL UNIQUE,plan_hash TEXT NOT NULL,connection TEXT NOT NULL DEFAULT '{}',repository_path TEXT NOT NULL,files TEXT NOT NULL,diff TEXT NOT NULL,preview_hash TEXT NOT NULL,prepared_by TEXT NOT NULL,created_at TEXT NOT NULL,FOREIGN KEY(plan_id) REFERENCES component_lifecycle_execution_plans(id))`,
		`CREATE INDEX IF NOT EXISTS idx_component_gitops_previews_created ON component_gitops_previews(created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS component_gitops_previews_no_update BEFORE UPDATE ON component_gitops_previews BEGIN SELECT RAISE(ABORT,'component GitOps previews are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS component_gitops_previews_no_delete BEFORE DELETE ON component_gitops_previews BEGIN SELECT RAISE(ABORT,'component GitOps previews are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS gitops_preview_approvals(id TEXT PRIMARY KEY,preview_id TEXT NOT NULL UNIQUE,preview_hash TEXT NOT NULL,plan_hash TEXT NOT NULL,connection_id TEXT NOT NULL,repository_path TEXT NOT NULL,branch TEXT NOT NULL,submitted_by TEXT NOT NULL,assigned_reviewer TEXT NOT NULL,submission_comment TEXT NOT NULL,status TEXT NOT NULL,reviewed_by TEXT NOT NULL DEFAULT '',review_comment TEXT NOT NULL DEFAULT '',reviewed_at TEXT,created_at TEXT NOT NULL,FOREIGN KEY(preview_id) REFERENCES component_gitops_previews(id))`,
		`CREATE INDEX IF NOT EXISTS idx_gitops_preview_approvals_status ON gitops_preview_approvals(status,created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS gitops_execution_requests(id TEXT PRIMARY KEY,approval_id TEXT NOT NULL UNIQUE,preview_id TEXT NOT NULL,preview_hash TEXT NOT NULL,plan_hash TEXT NOT NULL,connection_id TEXT NOT NULL,provider TEXT NOT NULL,mode TEXT NOT NULL,repository_path TEXT NOT NULL,branch TEXT NOT NULL,requested_by TEXT NOT NULL,created_at TEXT NOT NULL,FOREIGN KEY(approval_id) REFERENCES gitops_preview_approvals(id),FOREIGN KEY(preview_id) REFERENCES component_gitops_previews(id))`,
		`CREATE INDEX IF NOT EXISTS idx_gitops_execution_requests_created ON gitops_execution_requests(created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS gitops_execution_requests_no_update BEFORE UPDATE ON gitops_execution_requests BEGIN SELECT RAISE(ABORT,'GitOps execution requests are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS gitops_execution_requests_no_delete BEFORE DELETE ON gitops_execution_requests BEGIN SELECT RAISE(ABORT,'GitOps execution requests are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS gitops_execution_events(id TEXT PRIMARY KEY,execution_id TEXT NOT NULL,status TEXT NOT NULL,actor TEXT NOT NULL,message TEXT NOT NULL,evidence TEXT NOT NULL DEFAULT '{}',created_at TEXT NOT NULL,FOREIGN KEY(execution_id) REFERENCES gitops_execution_requests(id))`,
		`CREATE INDEX IF NOT EXISTS idx_gitops_execution_events_execution ON gitops_execution_events(execution_id,created_at,id)`,
		`CREATE TRIGGER IF NOT EXISTS gitops_execution_events_no_update BEFORE UPDATE ON gitops_execution_events BEGIN SELECT RAISE(ABORT,'GitOps execution events are append-only'); END`,
		`CREATE TRIGGER IF NOT EXISTS gitops_execution_events_no_delete BEFORE DELETE ON gitops_execution_events BEGIN SELECT RAISE(ABORT,'GitOps execution events are append-only'); END`,
		`CREATE TABLE IF NOT EXISTS gitops_execution_leases(execution_id TEXT PRIMARY KEY,worker_id TEXT NOT NULL,lease_until TEXT NOT NULL,claimed_at TEXT NOT NULL,FOREIGN KEY(execution_id) REFERENCES gitops_execution_requests(id))`,
		`CREATE INDEX IF NOT EXISTS idx_gitops_execution_leases_until ON gitops_execution_leases(lease_until)`,
		`CREATE TABLE IF NOT EXISTS integration_connections(id TEXT PRIMARY KEY,name TEXT NOT NULL UNIQUE,provider TEXT NOT NULL,base_url TEXT NOT NULL DEFAULT '',organization TEXT NOT NULL,project TEXT NOT NULL DEFAULT '',repository TEXT NOT NULL,branch TEXT NOT NULL,allowed_root TEXT NOT NULL,mode TEXT NOT NULL,credential_ref TEXT NOT NULL,group_ids TEXT NOT NULL,enabled INTEGER NOT NULL,created_by TEXT NOT NULL,created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_integration_connections_provider ON integration_connections(provider,created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS integration_connections_no_update BEFORE UPDATE ON integration_connections BEGIN SELECT RAISE(ABORT,'integration connection records are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS integration_connections_no_delete BEFORE DELETE ON integration_connections BEGIN SELECT RAISE(ABORT,'integration connection records are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS integration_connection_validations(id TEXT PRIMARY KEY,connection_id TEXT NOT NULL,provider TEXT NOT NULL,status TEXT NOT NULL,message TEXT NOT NULL,evidence TEXT NOT NULL DEFAULT '{}',validated_by TEXT NOT NULL,created_at TEXT NOT NULL,expires_at TEXT NOT NULL,FOREIGN KEY(connection_id) REFERENCES integration_connections(id))`,
		`CREATE INDEX IF NOT EXISTS idx_integration_connection_validations_connection ON integration_connection_validations(connection_id,created_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS integration_connection_validations_no_update BEFORE UPDATE ON integration_connection_validations BEGIN SELECT RAISE(ABORT,'integration validation records are immutable'); END`,
		`CREATE TRIGGER IF NOT EXISTS integration_connection_validations_no_delete BEFORE DELETE ON integration_connection_validations BEGIN SELECT RAISE(ABORT,'integration validation records are immutable'); END`,
		`CREATE TABLE IF NOT EXISTS administrators (
            singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
            username TEXT NOT NULL UNIQUE, role TEXT NOT NULL DEFAULT 'admin', password_salt BLOB NOT NULL,
            password_hash BLOB NOT NULL, created_at TEXT NOT NULL,
            password_changed_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS users (
            username TEXT PRIMARY KEY COLLATE NOCASE,
            email TEXT NOT NULL DEFAULT '',
            role TEXT NOT NULL CHECK (role IN ('admin','operator','group_owner','viewer')),
			enabled INTEGER NOT NULL DEFAULT 1,
			group_ids TEXT NOT NULL DEFAULT '[]',
			timezone TEXT NOT NULL DEFAULT 'UTC',
			last_login_at TEXT,
			must_change_password INTEGER NOT NULL DEFAULT 0,
            password_salt BLOB NOT NULL, password_hash BLOB NOT NULL,
            created_at TEXT NOT NULL, password_changed_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_users_role_enabled ON users(role, enabled)`,
		`CREATE TABLE IF NOT EXISTS password_recovery_tokens (
			username TEXT PRIMARY KEY COLLATE NOCASE,
			token_hash BLOB NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS configuration_section_policies (
            section_key TEXT PRIMARY KEY,
            operator_editable INTEGER NOT NULL CHECK (operator_editable IN (0, 1)),
            updated_at TEXT NOT NULL
        )`,
		`CREATE TABLE IF NOT EXISTS configuration_drift_policy (
            singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
            policy TEXT NOT NULL CHECK (policy IN ('report_only','enforce')),
            updated_at TEXT NOT NULL
        )`,
		`INSERT OR IGNORE INTO configuration_drift_policy(singleton,policy,updated_at)
            VALUES(1,'report_only',CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            occurred_at TEXT NOT NULL, actor TEXT NOT NULL,
            action TEXT NOT NULL, resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL DEFAULT '', outcome TEXT NOT NULL, details TEXT NOT NULL DEFAULT '',
            http_method TEXT NOT NULL, path TEXT NOT NULL, status_code INTEGER NOT NULL
        )`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_occurred ON audit_events(occurred_at DESC,id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_actor ON audit_events(actor,occurred_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events(action,occurred_at DESC)`,
		`CREATE TABLE IF NOT EXISTS guide_catalog_versions (version INTEGER PRIMARY KEY, content TEXT NOT NULL, created_by TEXT NOT NULL, created_at TEXT NOT NULL, published_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS guide_catalog_state (singleton INTEGER PRIMARY KEY CHECK(singleton=1), active_version INTEGER NOT NULL, FOREIGN KEY(active_version) REFERENCES guide_catalog_versions(version))`,
		`CREATE TABLE IF NOT EXISTS guide_catalog_draft (singleton INTEGER PRIMARY KEY CHECK(singleton=1), content TEXT NOT NULL, base_version INTEGER NOT NULL, updated_by TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := d.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize sqlite schema: %w", err)
		}
	}
	if err := d.ensureGroupEnabledColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureConfigurationGroupColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureDestinationGovernanceColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureGroupOwnersColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureGroupMembershipRuleColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureApprovalReviewColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureApprovalGovernanceColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureDeploymentRollbackColumns(ctx); err != nil {
		return err
	}
	if err := d.ensureMembershipExecutionResultsColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureAdministratorRoleColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureGroupOwnerRole(ctx); err != nil {
		return err
	}
	if err := d.ensureUserEmailColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureUserProfileColumns(ctx); err != nil {
		return err
	}
	if err := d.ensurePasswordRecoverySchema(ctx); err != nil {
		return err
	}
	if err := d.ensureRBACSchema(ctx); err != nil {
		return err
	}
	if err := d.ensureAuditDetailsColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureGitOpsPreviewConnectionColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureDistributionBaselineColumn(ctx); err != nil {
		return err
	}
	if err := d.ensureArtifactPublicationRetrySchema(ctx); err != nil {
		return err
	}
	if err := d.ensureDistributionPreviewReleaseColumns(ctx); err != nil {
		return err
	}
	if err := d.migrateAdministratorToUsers(ctx); err != nil {
		return err
	}
	return d.db.PingContext(ctx)
}

func (d *Database) ensureDistributionPreviewReleaseColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"release_id", "TEXT NOT NULL DEFAULT ''"},
		{"release_evidence_hash", "TEXT NOT NULL DEFAULT ''"},
		{"artifact_uri", "TEXT NOT NULL DEFAULT ''"},
		{"artifact_sha256", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "distribution_deployment_previews", column.name)
		if err != nil {
			return err
		}
		if !present {
			if _, err = d.db.ExecContext(ctx, `ALTER TABLE distribution_deployment_previews ADD COLUMN `+column.name+` `+column.definition); err != nil {
				return fmt.Errorf("add distribution_deployment_previews.%s: %w", column.name, err)
			}
		}
	}
	return nil
}

func (d *Database) ensureArtifactPublicationRetrySchema(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "distribution_artifact_publication_executions", "attempt_number")
	if err != nil || present {
		return err
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`DROP TRIGGER IF EXISTS distribution_artifact_publication_executions_no_update`,
		`DROP TRIGGER IF EXISTS distribution_artifact_publication_executions_no_delete`,
		`DROP INDEX IF EXISTS idx_distribution_artifact_publication_executions_time`,
		`CREATE TABLE distribution_artifact_publication_executions_next (
			id TEXT PRIMARY KEY, publication_id TEXT NOT NULL, publication_evidence_hash TEXT NOT NULL,
			preflight_id TEXT NOT NULL, preflight_evidence_hash TEXT NOT NULL,
			artifact_id TEXT NOT NULL, artifact_sha256 TEXT NOT NULL, destination_ref TEXT NOT NULL, credential_ref TEXT NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('succeeded','failed')), failure_code TEXT NOT NULL, failure_digest TEXT NOT NULL,
			attempt_number INTEGER NOT NULL, trigger TEXT NOT NULL CHECK(trigger IN ('automatic','manual')), retry_request_id TEXT NOT NULL,
			worker_id TEXT NOT NULL, evidence_hash TEXT NOT NULL UNIQUE, started_at TEXT NOT NULL, completed_at TEXT NOT NULL,
			UNIQUE(publication_id,attempt_number), FOREIGN KEY(publication_id) REFERENCES distribution_artifact_publications(id),
			FOREIGN KEY(preflight_id) REFERENCES distribution_artifact_publication_preflights(id))`,
		`INSERT INTO distribution_artifact_publication_executions_next(id,publication_id,publication_evidence_hash,preflight_id,preflight_evidence_hash,artifact_id,artifact_sha256,destination_ref,credential_ref,status,failure_code,failure_digest,attempt_number,trigger,retry_request_id,worker_id,evidence_hash,started_at,completed_at) SELECT id,publication_id,publication_evidence_hash,preflight_id,preflight_evidence_hash,artifact_id,artifact_sha256,destination_ref,credential_ref,status,failure_code,failure_digest,1,'automatic','',worker_id,evidence_hash,started_at,completed_at FROM distribution_artifact_publication_executions`,
		`DROP TABLE distribution_artifact_publication_executions`,
		`ALTER TABLE distribution_artifact_publication_executions_next RENAME TO distribution_artifact_publication_executions`,
		`CREATE INDEX idx_distribution_artifact_publication_executions_time ON distribution_artifact_publication_executions(completed_at DESC)`,
		`CREATE TRIGGER distribution_artifact_publication_executions_no_update BEFORE UPDATE ON distribution_artifact_publication_executions BEGIN SELECT RAISE(ABORT,'Distribution artifact publication executions are immutable'); END`,
		`CREATE TRIGGER distribution_artifact_publication_executions_no_delete BEFORE DELETE ON distribution_artifact_publication_executions BEGIN SELECT RAISE(ABORT,'Distribution artifact publication executions are immutable'); END`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate artifact publication retry schema: %w", err)
		}
	}
	return tx.Commit()
}

func (d *Database) ensureDistributionBaselineColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "collector_distributions", "baseline_config")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE collector_distributions ADD COLUMN baseline_config TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add collector_distributions.baseline_config column: %w", err)
	}
	return nil
}

func (d *Database) ensureGitOpsPreviewConnectionColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "component_gitops_previews", "connection")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE component_gitops_previews ADD COLUMN connection TEXT NOT NULL DEFAULT '{}'`); err != nil {
		return fmt.Errorf("add component_gitops_previews.connection column: %w", err)
	}
	return nil
}

func (d *Database) ensureDestinationGovernanceColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"owner", "TEXT NOT NULL DEFAULT 'Platform Team'"},
		{"visibility", "TEXT NOT NULL DEFAULT 'organization'"},
		{"group_ids", "TEXT NOT NULL DEFAULT '[]'"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "destination_profiles", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE destination_profiles ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add destination_profiles.%s column: %w", column.name, err)
		}
	}
	return nil
}

func (d *Database) ensureUserProfileColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"group_ids", "TEXT NOT NULL DEFAULT '[]'"},
		{"timezone", "TEXT NOT NULL DEFAULT 'UTC'"},
		{"last_login_at", "TEXT"},
		{"must_change_password", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "users", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add users.%s column: %w", column.name, err)
		}
	}
	return nil
}

func (d *Database) ensurePasswordRecoverySchema(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS password_recovery_tokens (
		username TEXT PRIMARY KEY COLLATE NOCASE,
		token_hash BLOB NOT NULL,
		expires_at TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY(username) REFERENCES users(username) ON DELETE CASCADE
	)`)
	if err != nil {
		return fmt.Errorf("create password recovery token table: %w", err)
	}
	return nil
}

func (d *Database) ensureAuditDetailsColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "audit_events", "details")
	if err != nil || present {
		return err
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE audit_events ADD COLUMN details TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add audit_events.details column: %w", err)
	}
	return nil
}

func (d *Database) ensureConfigurationGroupColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "configurations", "group_id")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE configurations ADD COLUMN group_id TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add configurations.group_id column: %w", err)
	}
	return nil
}

func (d *Database) ensureUserEmailColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "users", "email")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("add users.email column: %w", err)
	}
	return nil
}

// ensureGroupOwnerRole widens the users role constraint for databases created
// before resource-scoped group owners were introduced.
func (d *Database) ensureGroupOwnerRole(ctx context.Context) error {
	var schema string
	if err := d.db.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&schema); err != nil {
		return fmt.Errorf("inspect users role constraint: %w", err)
	}
	if strings.Contains(schema, "group_owner") {
		return nil
	}
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		`CREATE TABLE users_next (
            username TEXT PRIMARY KEY COLLATE NOCASE,
            role TEXT NOT NULL CHECK (role IN ('admin','operator','group_owner','viewer')),
            enabled INTEGER NOT NULL DEFAULT 1,
            password_salt BLOB NOT NULL, password_hash BLOB NOT NULL,
            created_at TEXT NOT NULL, password_changed_at TEXT NOT NULL,
            updated_at TEXT NOT NULL
        )`,
		`INSERT INTO users_next SELECT username,role,enabled,password_salt,password_hash,created_at,password_changed_at,updated_at FROM users`,
		`DROP TABLE users`,
		`ALTER TABLE users_next RENAME TO users`,
		`CREATE INDEX idx_users_role_enabled ON users(role, enabled)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate users group_owner role: %w", err)
		}
	}
	return tx.Commit()
}

func (d *Database) ensureMembershipExecutionResultsColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "membership_changes", "execution_results")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE membership_changes ADD COLUMN execution_results TEXT NOT NULL DEFAULT '[]'`); err != nil {
		return fmt.Errorf("add membership_changes.execution_results column: %w", err)
	}
	return nil
}

func (d *Database) ensureDeploymentRollbackColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"previous_configuration_id", "TEXT NOT NULL DEFAULT ''"},
		{"approval_request_id", "TEXT NOT NULL DEFAULT ''"},
		{"rollback_started_at", "TEXT"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "deployments", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE deployments ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add deployments.%s column: %w", column.name, err)
		}
	}
	return nil
}

func (d *Database) ensureApprovalReviewColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"label_selector", "TEXT NOT NULL DEFAULT '{}'"},
		{"base_configuration_id", "TEXT NOT NULL DEFAULT ''"},
		{"base_configuration_hash", "TEXT NOT NULL DEFAULT ''"},
		{"reviewed_by", "TEXT NOT NULL DEFAULT ''"},
		{"review_comment", "TEXT NOT NULL DEFAULT ''"},
		{"reviewed_at", "TEXT"},
		{"expires_at", "TEXT NOT NULL DEFAULT '9999-12-31T23:59:59Z'"},
		{"expired_at", "TEXT"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "group_deployment_requests", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE group_deployment_requests ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add group_deployment_requests.%s column: %w", column.name, err)
		}
	}
	if _, err := d.db.ExecContext(ctx, `UPDATE group_deployment_requests
        SET expires_at=strftime('%Y-%m-%dT%H:%M:%fZ',created_at,'+30 days')
        WHERE expires_at='9999-12-31T23:59:59Z'`); err != nil {
		return fmt.Errorf("backfill approval expiration: %w", err)
	}
	return nil
}

func (d *Database) ensureApprovalGovernanceColumns(ctx context.Context) error {
	columns := []struct{ name, definition string }{
		{"assigned_reviewer", "TEXT NOT NULL DEFAULT ''"},
		{"change_reason", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, column := range columns {
		present, err := sqliteColumnExists(ctx, d.db, "group_deployment_requests", column.name)
		if err != nil {
			return err
		}
		if present {
			continue
		}
		if _, err := d.db.ExecContext(ctx, `ALTER TABLE group_deployment_requests ADD COLUMN `+column.name+` `+column.definition); err != nil {
			return fmt.Errorf("add group_deployment_requests.%s column: %w", column.name, err)
		}
	}
	return nil
}

func sqliteColumnExists(ctx context.Context, db *sql.DB, table, wanted string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == wanted {
			return true, nil
		}
	}
	return false, rows.Err()
}

// ensureGroupOwnersColumn migrates existing installations to resource-scoped group ownership.
func (d *Database) ensureGroupOwnersColumn(ctx context.Context) error {
	rows, err := d.db.QueryContext(ctx, `PRAGMA table_info(groups)`)
	if err != nil {
		return fmt.Errorf("inspect groups schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "owners" {
			return nil
		}
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE groups ADD COLUMN owners TEXT NOT NULL DEFAULT '[]'`); err != nil {
		return fmt.Errorf("add groups.owners column: %w", err)
	}
	return nil
}

func (d *Database) ensureGroupMembershipRuleColumn(ctx context.Context) error {
	present, err := sqliteColumnExists(ctx, d.db, "groups", "membership_rule")
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE groups ADD COLUMN membership_rule TEXT NOT NULL DEFAULT 'null'`); err != nil {
		return fmt.Errorf("add groups.membership_rule column: %w", err)
	}
	return nil
}

// ensureGroupEnabledColumn migrates older group tables that predate the enabled flag.
func (d *Database) ensureGroupEnabledColumn(ctx context.Context) error {
	rows, err := d.db.QueryContext(ctx, `PRAGMA table_info(groups)`)
	if err != nil {
		return fmt.Errorf("inspect groups schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "enabled" {
			return nil
		}
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE groups ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1`); err != nil {
		return fmt.Errorf("add groups.enabled column: %w", err)
	}
	return nil
}

// ensureAdministratorRoleColumn promotes the existing first-login account to
// Admin while making its authorization role explicit for RBAC-aware sessions.
func (d *Database) ensureAdministratorRoleColumn(ctx context.Context) error {
	rows, err := d.db.QueryContext(ctx, `PRAGMA table_info(administrators)`)
	if err != nil {
		return fmt.Errorf("inspect administrators schema: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "role" {
			return nil
		}
	}
	if _, err := d.db.ExecContext(ctx, `ALTER TABLE administrators ADD COLUMN role TEXT NOT NULL DEFAULT 'admin'`); err != nil {
		return fmt.Errorf("add administrators.role column: %w", err)
	}
	return nil
}

// migrateAdministratorToUsers preserves the bootstrap account while moving
// authentication to the multi-user RBAC store. It is safe to run repeatedly.
func (d *Database) migrateAdministratorToUsers(ctx context.Context) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin administrator migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
        INSERT OR IGNORE INTO users (
            username, role, enabled, password_salt, password_hash,
            created_at, password_changed_at, updated_at
        )
        SELECT username, role, 1, password_salt, password_hash,
               created_at, password_changed_at, password_changed_at
        FROM administrators
    `); err != nil {
		return fmt.Errorf("migrate administrator to users: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM administrators`); err != nil {
		return fmt.Errorf("clear migrated administrator credential: %w", err)
	}
	return tx.Commit()
}
