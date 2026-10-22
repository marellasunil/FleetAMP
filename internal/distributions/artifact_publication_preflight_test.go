package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type publicationSecretResolver struct{ err error }

func (r publicationSecretResolver) Resolve(context.Context, string) (string, error) {
	if r.err != nil {
		return "", r.err
	}
	return "temporary-secret", nil
}

func TestArtifactPublicationPreflightVerifiesExactEvidence(t *testing.T) {
	root := t.TempDir()
	content := []byte("fleetamp collector artifact")
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])
	dir := filepath.Join(root, sha)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fleetamp-collector"), content, 0o500); err != nil {
		t.Fatal(err)
	}
	artifact := &BuildArtifact{ID: "artifact-1", ArtifactReference: "artifact-cache://" + sha + "/fleetamp-collector", ArtifactSHA256: sha, ArtifactSize: int64(len(content)), Status: BuildArtifactGenerated}
	publication := &ArtifactPublication{ID: "publication-1", ArtifactID: artifact.ID, ArtifactSHA256: sha, DestinationRef: "oci://registry.example.test/fleetamp/collector:v1", CredentialRef: "secret://registry/production", Status: ArtifactPublicationQueued, EvidenceHash: strings.Repeat("a", 64)}
	verifier := &ArtifactPublicationPreflightVerifier{ArtifactRoot: root, AllowedHosts: []string{"registry.example.test"}, Secrets: publicationSecretResolver{}}
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	receipt, err := verifier.Verify(context.Background(), publication, artifact, "admin", now)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != ArtifactPublicationPreflightPassed || !receipt.CredentialAvailable || receipt.DestinationHost != "registry.example.test" || receipt.ArtifactSHA256 != sha || receipt.ArtifactSize != int64(len(content)) || receipt.PublicationEvidenceHash != publication.EvidenceHash {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if strings.Contains(receipt.EvidenceHash, "temporary-secret") {
		t.Fatal("receipt must not expose credential value")
	}
}

func TestArtifactPublicationPreflightRejectsUnsafeInputs(t *testing.T) {
	root := t.TempDir()
	content := []byte("artifact")
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])
	dir := filepath.Join(root, sha)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fleetamp-collector"), content, 0o500); err != nil {
		t.Fatal(err)
	}
	baseArtifact := &BuildArtifact{ID: "artifact-1", ArtifactReference: "artifact-cache://" + sha + "/fleetamp-collector", ArtifactSHA256: sha, ArtifactSize: int64(len(content)), Status: BuildArtifactGenerated}
	basePublication := &ArtifactPublication{ID: "publication-1", ArtifactID: baseArtifact.ID, ArtifactSHA256: sha, DestinationRef: "oci://registry.example.test/fleetamp/collector:v1", CredentialRef: "secret://registry/production", Status: ArtifactPublicationQueued, EvidenceHash: strings.Repeat("a", 64)}
	tests := []struct {
		name   string
		mutate func(*ArtifactPublication, *BuildArtifact, *ArtifactPublicationPreflightVerifier)
	}{
		{"mismatched artifact", func(p *ArtifactPublication, _ *BuildArtifact, _ *ArtifactPublicationPreflightVerifier) {
			p.ArtifactSHA256 = strings.Repeat("b", 64)
		}},
		{"path traversal", func(_ *ArtifactPublication, a *BuildArtifact, _ *ArtifactPublicationPreflightVerifier) {
			a.ArtifactReference = "artifact-cache://../secret"
		}},
		{"digest changed", func(_ *ArtifactPublication, a *BuildArtifact, _ *ArtifactPublicationPreflightVerifier) {
			a.ArtifactSHA256 = strings.Repeat("c", 64)
		}},
		{"host not allowed", func(p *ArtifactPublication, _ *BuildArtifact, _ *ArtifactPublicationPreflightVerifier) {
			p.DestinationRef = "oci://evil.example.test/fleetamp:v1"
		}},
		{"credential missing", func(_ *ArtifactPublication, _ *BuildArtifact, v *ArtifactPublicationPreflightVerifier) {
			v.Secrets = publicationSecretResolver{err: errors.New("missing")}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifact, publication := *baseArtifact, *basePublication
			verifier := &ArtifactPublicationPreflightVerifier{ArtifactRoot: root, AllowedHosts: []string{"registry.example.test"}, Secrets: publicationSecretResolver{}}
			test.mutate(&publication, &artifact, verifier)
			if _, err := verifier.Verify(context.Background(), &publication, &artifact, "admin", time.Now()); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestArtifactPublicationPreflightRejectsSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "fleetamp-collector"), []byte("secret"), 0o500); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	artifact := &BuildArtifact{ID: "artifact-1", ArtifactReference: "artifact-cache://linked/fleetamp-collector", ArtifactSHA256: strings.Repeat("a", 64), ArtifactSize: 6, Status: BuildArtifactGenerated}
	publication := &ArtifactPublication{ID: "publication-1", ArtifactID: artifact.ID, ArtifactSHA256: artifact.ArtifactSHA256, DestinationRef: "oci://registry.example.test/fleetamp:v1", CredentialRef: "secret://registry", Status: ArtifactPublicationQueued, EvidenceHash: strings.Repeat("b", 64)}
	verifier := &ArtifactPublicationPreflightVerifier{ArtifactRoot: root, AllowedHosts: []string{"registry.example.test"}, Secrets: publicationSecretResolver{}}
	if _, err := verifier.Verify(context.Background(), publication, artifact, "admin", time.Now()); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}
