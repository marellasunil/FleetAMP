package distributions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testBuilderImage = "example.invalid/fleetamp/ocb-builder@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func inspectedBuildSource() *BuildSourceInspection {
	manifest := "dist:\n  name: fleetamp-collector\n  output_path: ./_build\n"
	sum := sha256.Sum256([]byte(manifest))
	return &BuildSourceInspection{ID: "inspection-1", ExecutionID: "execution-1", BuildPlanHash: strings.Repeat("b", 64), ManifestYAML: manifest, ManifestSHA256: hex.EncodeToString(sum[:]), Status: BuildSourceInspected}
}

func TestBuildArtifactBindsExactInspectionAndDigestPinnedBuilder(t *testing.T) {
	inspection := inspectedBuildSource()
	result := &OCBRunResult{Runtime: "podman", BuilderImage: testBuilderImage, ArtifactReference: "artifact-cache://" + strings.Repeat("d", 64) + "/fleetamp-collector", ArtifactSHA256: strings.Repeat("d", 64), ArtifactSize: 8}
	one, err := NewBuildArtifact(inspection, result, "worker", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewBuildArtifact(inspection, result, "other", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if one.ID != two.ID || one.EvidenceHash != two.EvidenceHash {
		t.Fatalf("evidence is not deterministic: %#v %#v", one, two)
	}
	result.BuilderImage = "example.invalid/builder:latest"
	if _, err := NewBuildArtifact(inspection, result, "worker", time.Now()); err == nil {
		t.Fatal("expected mutable builder image rejection")
	}
}

func TestContainerOCBRunnerUsesConstrainedOfflineContainer(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "args.log")
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FLEETAMP_TEST_ARGS"
for arg in "$@"; do
  case "$arg" in
    type=bind,src=*,dst=/workspace,rw) workspace=${arg#type=bind,src=}; workspace=${workspace%,dst=/workspace,rw} ;;
  esac
done
mkdir -p "$workspace/_build"
printf 'collector-binary' > "$workspace/_build/fleetamp-collector"
`
	runtimePath := filepath.Join(binDir, "podman")
	if err := os.WriteFile(runtimePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FLEETAMP_TEST_ARGS", logPath)
	root := t.TempDir()
	runner := &ContainerOCBRunner{Runtime: "podman", BuilderImage: testBuilderImage, WorkspaceRoot: filepath.Join(root, "work"), ArtifactRoot: filepath.Join(root, "artifacts"), MaxArtifactBytes: 1024}
	result, err := runner.Build(context.Background(), inspectedBuildSource())
	if err != nil {
		t.Fatal(err)
	}
	if result.ArtifactSize != 16 || !strings.HasPrefix(result.ArtifactReference, "artifact-cache://") {
		t.Fatalf("unexpected result: %#v", result)
	}
	args, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(args)
	for _, required := range []string{"--network=none", "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--pids-limit=256", "--memory=1g", "--cpus=1", "GOPROXY=off", "GOSUMDB=off", "GOMODCACHE=/opt/fleetamp/gomodcache", testBuilderImage} {
		if !strings.Contains(text, required) {
			t.Errorf("missing constrained argument %q in:\n%s", required, text)
		}
	}
}

func TestContainerOCBRunnerRejectsUnpinnedImage(t *testing.T) {
	runner := &ContainerOCBRunner{Runtime: "podman", BuilderImage: "example.invalid/builder:latest", WorkspaceRoot: t.TempDir(), ArtifactRoot: t.TempDir()}
	if _, err := runner.Build(context.Background(), inspectedBuildSource()); err == nil {
		t.Fatal("expected unpinned image rejection")
	}
}

func TestContainerOCBRunnerRejectsChangedManifestEvidence(t *testing.T) {
	inspection := inspectedBuildSource()
	inspection.ManifestYAML += "# changed after inspection\n"
	runner := &ContainerOCBRunner{Runtime: "podman", BuilderImage: testBuilderImage, WorkspaceRoot: t.TempDir(), ArtifactRoot: t.TempDir()}
	if _, err := runner.Build(context.Background(), inspection); err == nil || !strings.Contains(err.Error(), "immutable inspection evidence") {
		t.Fatalf("unexpected error: %v", err)
	}
}
