package distributions

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testExecution(t *testing.T) *BuildExecution {
	t.Helper()
	m := BuildManifest{CollectorVersion: "0.140.0", Components: ComponentSet{Receivers: []string{"otlp"}, Processors: []string{"batch"}, Exporters: []string{"debug"}, Extensions: []string{"health_check"}}}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return &BuildExecution{ID: "execution-1", BuildPlanHash: strings.Repeat("a", 64), ManifestJSON: string(raw)}
}

func writeArchive(t *testing.T, dir string, entries []tar.Header) (string, int64) {
	t.Helper()
	tmp := filepath.Join(dir, "pending.tar.gz")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, header := range entries {
		h := header
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg && h.Size > 0 {
			if _, err := tw.Write([]byte(strings.Repeat("x", int(h.Size)))); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	final := filepath.Join(dir, sha+".tar.gz")
	if err := os.Rename(tmp, final); err != nil {
		t.Fatal(err)
	}
	return sha, int64(len(raw))
}

func TestInspectBuildSourceWithoutExtractionAndGenerateManifest(t *testing.T) {
	dir := t.TempDir()
	sha, size := writeArchive(t, dir, []tar.Header{{Name: "collector-0.140.0/", Typeflag: tar.TypeDir, Mode: 0755}, {Name: "collector-0.140.0/README.md", Typeflag: tar.TypeReg, Mode: 0644, Size: 4}})
	execution := testExecution(t)
	acquisition := &BuildSourceAcquisition{ID: "acq-1", ExecutionID: execution.ID, BuildPlanHash: execution.BuildPlanHash, ArchiveSHA256: sha, LocalReference: "source-cache://" + sha + ".tar.gz", Status: BuildSourceAcquired}
	item, err := InspectBuildSource(acquisition, execution, dir, "inspector", ArchiveInspectionLimits{}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if item.FileCount != 1 || item.RootPrefix != "collector-0.140.0" || item.ManifestSHA256 == "" {
		t.Fatalf("unexpected inspection: %#v", item)
	}
	if !strings.Contains(item.ManifestYAML, "otlpreceiver v0.140.0") || !strings.Contains(item.ManifestYAML, "batchprocessor v0.140.0") {
		t.Fatalf("unexpected manifest:\n%s", item.ManifestYAML)
	}
	_ = size
}

func TestInspectBuildSourceRejectsTraversalAndLinks(t *testing.T) {
	for name, header := range map[string]tar.Header{"traversal": {Name: "../escape", Typeflag: tar.TypeReg, Size: 1}, "symlink": {Name: "root/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			sha, _ := writeArchive(t, dir, []tar.Header{header})
			execution := testExecution(t)
			acquisition := &BuildSourceAcquisition{ID: "acq", ExecutionID: execution.ID, BuildPlanHash: execution.BuildPlanHash, ArchiveSHA256: sha, LocalReference: "source-cache://" + sha + ".tar.gz", Status: BuildSourceAcquired}
			if _, err := InspectBuildSource(acquisition, execution, dir, "inspector", ArchiveInspectionLimits{}, time.Now()); err == nil {
				t.Fatal("expected unsafe archive rejection")
			}
		})
	}
}

func TestGenerateOCBManifestRejectsUnreviewedComponent(t *testing.T) {
	execution := testExecution(t)
	var m BuildManifest
	if err := json.Unmarshal([]byte(execution.ManifestJSON), &m); err != nil {
		t.Fatal(err)
	}
	m.Components.Exporters = []string{"unknown"}
	raw, _ := json.Marshal(m)
	execution.ManifestJSON = string(raw)
	if _, err := GenerateOCBManifest(execution); err == nil || !strings.Contains(err.Error(), "reviewed component catalog") {
		t.Fatalf("unexpected error: %v", err)
	}
}
