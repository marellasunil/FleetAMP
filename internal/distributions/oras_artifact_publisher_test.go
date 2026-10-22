package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestORASPublisherUsesStdinAndExactPreflightArtifact(t *testing.T) {
	root, staging := t.TempDir(), t.TempDir()
	content := []byte("collector-binary")
	sum := sha256.Sum256(content)
	sha := hex.EncodeToString(sum[:])
	artifactDir := filepath.Join(root, sha)
	if err := os.MkdirAll(artifactDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "fleetamp-collector"), content, 0o500); err != nil {
		t.Fatal(err)
	}
	argsFile, stdinFile := filepath.Join(t.TempDir(), "args"), filepath.Join(t.TempDir(), "stdin")
	script := filepath.Join(t.TempDir(), "oras-test")
	scriptBody := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\ncat > %q\n", argsFile, stdinFile)
	if err := os.WriteFile(script, []byte(scriptBody), 0o700); err != nil {
		t.Fatal(err)
	}
	preflight := &ArtifactPublicationPreflight{ArtifactReference: "artifact-cache://" + sha + "/fleetamp-collector", ArtifactSHA256: sha, ArtifactSize: int64(len(content)), DestinationRef: "oci://registry.example.test/fleetamp:v1", CredentialRef: "secret://registry", Status: ArtifactPublicationPreflightPassed}
	publisher := &ORASArtifactPublisher{Executable: script, ArtifactRoot: root, StagingRoot: staging, Username: "fleetamp", Secrets: publicationSecretResolver{}, MaxBytes: 1024}
	result, err := publisher.Publish(context.Background(), preflight)
	if err != nil {
		t.Fatal(err)
	}
	if result.ArtifactSHA256 != sha || result.DestinationRef != preflight.DestinationRef {
		t.Fatalf("unexpected result: %#v", result)
	}
	args, _ := os.ReadFile(argsFile)
	stdin, _ := os.ReadFile(stdinFile)
	if strings.Contains(string(args), "temporary-secret") || strings.TrimSpace(string(stdin)) != "temporary-secret" {
		t.Fatalf("credential transport was unsafe: args=%q stdin=%q", args, stdin)
	}
	entries, err := os.ReadDir(staging)
	if err != nil || len(entries) != 0 {
		t.Fatalf("staged artifact was not removed: %v, %v", entries, err)
	}
}
