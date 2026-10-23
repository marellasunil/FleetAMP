package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/configs"
	"github.com/marellasunil/FleetAMP/internal/distributions"
	"github.com/marellasunil/FleetAMP/internal/groups"
	"github.com/marellasunil/FleetAMP/internal/storage"
)

func TestDistributionApprovalHandoffCommitsAllRecords(t *testing.T) {
	ctx, db, group, preview := prepareDistributionHandoffTest(t)
	configuration, request, handoff := distributionHandoffRecords(t, group, preview, "1.0.0")
	if err := db.DistributionPreviewHandoffs().CreateApprovalHandoff(ctx, configuration, request, handoff); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Configurations().Get(ctx, configuration.ID); err != nil {
		t.Fatalf("configuration missing after commit: %v", err)
	}
	if _, err := db.GroupDeploymentRequests().Get(ctx, request.ID); err != nil {
		t.Fatalf("approval missing after commit: %v", err)
	}
	stored, err := db.DistributionPreviewHandoffs().GetByPreview(ctx, preview.ID)
	if err != nil || stored.EvidenceHash != handoff.EvidenceHash {
		t.Fatalf("handoff missing after commit: %#v %v", stored, err)
	}
}

func TestDistributionApprovalHandoffRollsBackEveryRecord(t *testing.T) {
	ctx, db, group, preview := prepareDistributionHandoffTest(t)
	firstConfig, firstRequest, firstHandoff := distributionHandoffRecords(t, group, preview, "1.0.0")
	if err := db.DistributionPreviewHandoffs().CreateApprovalHandoff(ctx, firstConfig, firstRequest, firstHandoff); err != nil {
		t.Fatal(err)
	}

	configuration, request, handoff := distributionHandoffRecords(t, group, preview, "1.0.1")
	if err := db.DistributionPreviewHandoffs().CreateApprovalHandoff(ctx, configuration, request, handoff); err == nil {
		t.Fatal("expected duplicate preview receipt to fail")
	}
	if _, err := db.Configurations().Get(ctx, configuration.ID); !errors.Is(err, storage.ErrConfigurationNotFound) {
		t.Fatalf("configuration survived rolled-back transaction: %v", err)
	}
	if _, err := db.GroupDeploymentRequests().Get(ctx, request.ID); !errors.Is(err, storage.ErrGroupDeploymentRequestNotFound) {
		t.Fatalf("approval survived rolled-back transaction: %v", err)
	}
	stored, err := db.DistributionPreviewHandoffs().GetByPreview(ctx, preview.ID)
	if err != nil || stored.ID != firstHandoff.ID {
		t.Fatalf("original receipt changed: %#v %v", stored, err)
	}
}

func TestDistributionApprovalHandoffRejectsMismatchedEvidenceBeforeWriting(t *testing.T) {
	ctx, db, group, preview := prepareDistributionHandoffTest(t)
	configuration, request, handoff := distributionHandoffRecords(t, group, preview, "1.0.0")
	handoff.ConfigurationHash = strings.Repeat("f", 64)
	if err := db.DistributionPreviewHandoffs().CreateApprovalHandoff(ctx, configuration, request, handoff); err == nil {
		t.Fatal("expected mismatched evidence rejection")
	}
	if _, err := db.Configurations().Get(ctx, configuration.ID); !errors.Is(err, storage.ErrConfigurationNotFound) {
		t.Fatalf("invalid configuration was written: %v", err)
	}
}

func prepareDistributionHandoffTest(t *testing.T) (context.Context, *Database, *groups.Group, *distributions.DeploymentPreview) {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "handoff.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	group, err := groups.New("Payments", "", map[string]string{"service": "payments"})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Groups().Create(ctx, group); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(10, 0).UTC()
	if _, err = db.db.ExecContext(ctx, `INSERT INTO collector_distributions(id,name,collector_version,source_repository,source_ref,artifact_uri,artifact_sha256,components,locked_sections,editable_sections,baseline_config,spec_hash,published_by,published_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, "dist-1", "Platform Collector", "0.140.0", "https://github.com/open-telemetry/opentelemetry-collector-releases", "v0.140.0", "oci://registry.example/platform:0.140.0", strings.Repeat("c", 64), `{}`, `[]`, `[]`, `receivers: {}`, strings.Repeat("a", 64), "admin", formatTime(now)); err != nil {
		t.Fatal(err)
	}
	content := "receivers: {}\n"
	configuration := configs.NewGroupConfiguration(group.ID, "seed", "0", content, "text/yaml")
	preview := &distributions.DeploymentPreview{ID: "preview-1", DistributionID: "dist-1", DistributionSpecHash: strings.Repeat("a", 64), ReleaseID: "release-1", ReleaseEvidenceHash: strings.Repeat("b", 64), ArtifactURI: "oci://registry.example/platform:0.140.0", ArtifactSHA256: strings.Repeat("c", 64), GroupID: group.ID, GroupName: group.Name, Sections: map[string]string{}, ComposedConfig: content, ContentHash: configuration.Hash, ValidationEvidence: `{"valid":true}`, PreparedBy: "owner", CreatedAt: now}
	if err = db.DistributionPreviews().Create(ctx, preview); err != nil {
		t.Fatal(err)
	}
	return ctx, db, group, preview
}

func distributionHandoffRecords(t *testing.T, group *groups.Group, preview *distributions.DeploymentPreview, version string) (*configs.Configuration, *configs.GroupDeploymentRequest, *distributions.PreviewHandoff) {
	t.Helper()
	configuration := configs.NewGroupConfiguration(group.ID, "payments-collector", version, preview.ComposedConfig, "text/yaml")
	request, err := configs.NewGroupDeploymentRequest(group.ID, group.Name, group.Selector, configuration, []configs.GroupDeploymentTarget{{AgentInstanceUID: "agent-1", AgentName: "payments-01", Readiness: "Ready", Eligible: true}}, "owner", "admin", "Deploy released package configuration")
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := distributions.NewPreviewHandoff(preview, configuration.ID, configuration.Hash, request.ID, "owner", time.Unix(20, 0))
	if err != nil {
		t.Fatal(err)
	}
	return configuration, request, handoff
}
