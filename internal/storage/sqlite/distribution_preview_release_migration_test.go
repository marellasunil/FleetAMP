package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marellasunil/FleetAMP/internal/distributions"
)

func TestDistributionPreviewReleaseMigrationPreservesLegacyRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-preview.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`CREATE TABLE distribution_deployment_previews (id TEXT PRIMARY KEY,distribution_id TEXT NOT NULL,distribution_spec_hash TEXT NOT NULL,group_id TEXT NOT NULL,group_name TEXT NOT NULL,sections TEXT NOT NULL,composed_config TEXT NOT NULL,content_hash TEXT NOT NULL,validation_evidence TEXT NOT NULL,prepared_by TEXT NOT NULL,created_at TEXT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = raw.Exec(`INSERT INTO distribution_deployment_previews VALUES('preview-1','dist-1','spec-1','group-1','Payments','{}','receivers: {}','content-1','{}','operator','2026-10-23T08:00:00Z')`)
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
	items, err := db.DistributionPreviews().List(context.Background(), 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("unexpected migrated previews: %#v %v", items, err)
	}
	if items[0].ReleaseID != "" || items[0].ArtifactSHA256 != "" {
		t.Fatalf("legacy preview received invented release evidence: %#v", items[0])
	}
}

func TestDistributionPreviewStorePersistsReleaseEvidence(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "preview.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.db.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		t.Fatal(err)
	}
	item := &distributions.DeploymentPreview{ID: "preview-1", DistributionID: "dist-1", DistributionSpecHash: strings.Repeat("a", 64), ReleaseID: "release-1", ReleaseEvidenceHash: strings.Repeat("b", 64), ArtifactURI: "oci://registry.example/collector:v1", ArtifactSHA256: strings.Repeat("c", 64), GroupID: "group-1", GroupName: "Payments", Sections: map[string]string{"receivers": "otlp: {}"}, ComposedConfig: "receivers: {}\n", ContentHash: strings.Repeat("d", 64), ValidationEvidence: "{\"valid\":true}", PreparedBy: "operator", CreatedAt: time.Unix(2, 0)}
	if err = db.DistributionPreviews().Create(ctx, item); err != nil {
		t.Fatal(err)
	}
	items, err := db.DistributionPreviews().List(ctx, 10)
	if err != nil || len(items) != 1 || items[0].ReleaseEvidenceHash != item.ReleaseEvidenceHash || items[0].ArtifactURI != item.ArtifactURI {
		t.Fatalf("unexpected previews: %#v %v", items, err)
	}
	if _, err = db.db.ExecContext(ctx, `UPDATE distribution_deployment_previews SET artifact_sha256='' WHERE id=?`, item.ID); err == nil {
		t.Fatal("immutable preview accepted update")
	}
}
