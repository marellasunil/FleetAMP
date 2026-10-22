package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ArtifactPublicationPreflightPassed = "passed"

type PublicationSecretResolver interface {
	Resolve(context.Context, string) (string, error)
}

// ArtifactPublicationPreflight is immutable evidence that the queued artifact,
// destination boundary and credential reference were safe immediately before a
// future publisher could consume the outbox record. It contains no secret.
type ArtifactPublicationPreflight struct {
	ID, PublicationID, PublicationEvidenceHash     string
	ArtifactID, ArtifactReference, ArtifactSHA256  string
	ArtifactSize                                   int64
	DestinationRef, DestinationHost, CredentialRef string
	CredentialAvailable                            bool
	Status, EvidenceHash, CheckedBy                string
	CheckedAt                                      time.Time
}

type ArtifactPublicationPreflightVerifier struct {
	ArtifactRoot string
	AllowedHosts []string
	Secrets      PublicationSecretResolver
	MaxBytes     int64
}

func (v *ArtifactPublicationPreflightVerifier) Verify(ctx context.Context, publication *ArtifactPublication, artifact *BuildArtifact, actor string, now time.Time) (*ArtifactPublicationPreflight, error) {
	if publication == nil || artifact == nil || publication.Status != ArtifactPublicationQueued {
		return nil, errors.New("queued publication and build artifact are required")
	}
	if publication.ArtifactID != artifact.ID || publication.ArtifactSHA256 != artifact.ArtifactSHA256 {
		return nil, errors.New("publication does not match exact artifact evidence")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil, errors.New("preflight actor is required")
	}
	relative, err := resolveArtifactRelativePath(artifact.ArtifactReference)
	if err != nil {
		return nil, err
	}
	sha, size, err := hashBoundedCachedArtifact(v.ArtifactRoot, relative, v.maxBytes())
	if err != nil {
		return nil, err
	}
	if sha != artifact.ArtifactSHA256 || size != artifact.ArtifactSize {
		return nil, errors.New("artifact content no longer matches immutable build evidence")
	}
	host, err := v.allowedDestinationHost(publication.DestinationRef)
	if err != nil {
		return nil, err
	}
	if v.Secrets == nil {
		return nil, errors.New("publication credential resolver is unavailable")
	}
	secret, err := v.Secrets.Resolve(ctx, publication.CredentialRef)
	if err != nil {
		return nil, fmt.Errorf("publication credential is unavailable: %w", err)
	}
	available := strings.TrimSpace(secret) != ""
	secret = ""
	if !available {
		return nil, errors.New("publication credential is unavailable")
	}
	payload := struct {
		PublicationID, PublicationEvidenceHash, ArtifactID, ArtifactReference, ArtifactSHA256 string
		ArtifactSize                                                                          int64
		DestinationRef, DestinationHost, CredentialRef                                        string
		CredentialAvailable                                                                   bool
		Status, Actor                                                                         string
		At                                                                                    time.Time
	}{publication.ID, publication.EvidenceHash, artifact.ID, artifact.ArtifactReference, sha, size, publication.DestinationRef, host, publication.CredentialRef, true, ArtifactPublicationPreflightPassed, actor, now.UTC()}
	canonical, _ := json.Marshal(payload)
	sum := sha256.Sum256(canonical)
	evidence := hex.EncodeToString(sum[:])
	return &ArtifactPublicationPreflight{ID: "artifact-publication-preflight-" + evidence[:16], PublicationID: publication.ID, PublicationEvidenceHash: publication.EvidenceHash, ArtifactID: artifact.ID, ArtifactReference: artifact.ArtifactReference, ArtifactSHA256: sha, ArtifactSize: size, DestinationRef: publication.DestinationRef, DestinationHost: host, CredentialRef: publication.CredentialRef, CredentialAvailable: true, Status: ArtifactPublicationPreflightPassed, EvidenceHash: evidence, CheckedBy: actor, CheckedAt: now.UTC()}, nil
}

func resolveArtifactRelativePath(reference string) (string, error) {
	relative := strings.TrimPrefix(strings.TrimSpace(reference), "artifact-cache://")
	if relative == reference || relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return "", errors.New("artifact reference is invalid")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("artifact reference escapes the cache")
	}
	return clean, nil
}

func (v *ArtifactPublicationPreflightVerifier) allowedDestinationHost(reference string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(reference))
	if err != nil || parsed.Scheme != "oci" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Trim(parsed.Path, "/") == "" {
		return "", errors.New("destination must be an oci:// registry reference")
	}
	host := strings.ToLower(parsed.Host)
	for _, allowed := range v.AllowedHosts {
		if strings.EqualFold(strings.TrimSpace(allowed), host) {
			return host, nil
		}
	}
	return "", fmt.Errorf("destination registry host %q is not allowlisted", host)
}

func (v *ArtifactPublicationPreflightVerifier) maxBytes() int64 {
	if v.MaxBytes <= 0 {
		return 512 << 20
	}
	return v.MaxBytes
}

func hashBoundedCachedArtifact(rootPath, relative string, maxBytes int64) (string, int64, error) {
	if strings.TrimSpace(rootPath) == "" {
		return "", 0, errors.New("artifact root is unavailable")
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return "", 0, fmt.Errorf("open artifact root: %w", err)
	}
	defer root.Close()
	info, err := root.Lstat(relative)
	if err != nil {
		return "", 0, fmt.Errorf("open cached artifact: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() <= 0 || info.Size() > maxBytes {
		return "", 0, errors.New("cached artifact is not a bounded regular file")
	}
	f, err := root.Open(relative)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	written, err := io.Copy(h, io.LimitReader(f, maxBytes+1))
	if err != nil || written != info.Size() {
		return "", 0, errors.New("cached artifact changed while reading")
	}
	return hex.EncodeToString(h.Sum(nil)), written, nil
}
