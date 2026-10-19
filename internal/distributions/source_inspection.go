package distributions

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const BuildSourceInspected = "source-inspected"

type BuildSourceInspection struct {
	ID, AcquisitionID, ExecutionID, BuildPlanHash                               string
	ArchiveSHA256                                                               string
	EntryCount, FileCount, DirectoryCount                                       int64
	UncompressedBytes                                                           int64
	RootPrefix, ManifestYAML, ManifestSHA256, Status, EvidenceHash, InspectedBy string
	InspectedAt                                                                 time.Time
}

type ArchiveInspectionLimits struct{ MaxEntries, MaxUncompressedBytes int64 }

func InspectBuildSource(acquisition *BuildSourceAcquisition, execution *BuildExecution, cacheDir, actor string, limits ArchiveInspectionLimits, now time.Time) (*BuildSourceInspection, error) {
	if acquisition == nil || execution == nil {
		return nil, errors.New("source acquisition and build execution are required")
	}
	if acquisition.Status != BuildSourceAcquired || acquisition.ExecutionID != execution.ID || acquisition.BuildPlanHash != execution.BuildPlanHash {
		return nil, errors.New("source acquisition does not match the exact build execution")
	}
	actor = strings.TrimSpace(actor)
	if actor == "" {
		return nil, errors.New("inspection actor is required")
	}
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = 100000
	}
	if limits.MaxUncompressedBytes <= 0 {
		limits.MaxUncompressedBytes = 1 << 30
	}
	name := strings.TrimPrefix(acquisition.LocalReference, "source-cache://")
	if name != acquisition.ArchiveSHA256+".tar.gz" || filepath.Base(name) != name {
		return nil, errors.New("source cache reference does not match the acquired archive")
	}
	cacheAbs, err := filepath.Abs(cacheDir)
	if err != nil {
		return nil, err
	}
	if filepath.Dir(filepath.Join(cacheAbs, name)) != cacheAbs {
		return nil, errors.New("source archive escapes the cache directory")
	}
	cacheRoot, err := os.OpenRoot(cacheAbs)
	if err != nil {
		return nil, fmt.Errorf("open source cache: %w", err)
	}
	defer cacheRoot.Close()
	f, err := cacheRoot.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open source archive: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	if hex.EncodeToString(h.Sum(nil)) != acquisition.ArchiveSHA256 {
		return nil, errors.New("source archive checksum changed after acquisition")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, errors.New("source archive is not a valid gzip stream")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	roots := map[string]bool{}
	var entries, files, dirs, bytes int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read source archive: %w", err)
		}
		entries++
		if entries > limits.MaxEntries {
			return nil, errors.New("source archive exceeds the entry limit")
		}
		clean := path.Clean(strings.ReplaceAll(hdr.Name, "\\", "/"))
		if clean == "." || strings.HasPrefix(clean, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("unsafe source archive path %q", hdr.Name)
		}
		roots[strings.Split(clean, "/")[0]] = true
		switch hdr.Typeflag {
		case tar.TypeDir:
			dirs++
		case tar.TypeReg, tar.TypeRegA:
			files++
			if hdr.Size < 0 || bytes > limits.MaxUncompressedBytes-hdr.Size {
				return nil, errors.New("source archive exceeds the uncompressed-size limit")
			}
			bytes += hdr.Size
		default:
			return nil, fmt.Errorf("unsafe source archive entry type for %q", hdr.Name)
		}
	}
	if entries == 0 || files == 0 {
		return nil, errors.New("source archive contains no regular files")
	}
	rootNames := make([]string, 0, len(roots))
	for root := range roots {
		rootNames = append(rootNames, root)
	}
	sort.Strings(rootNames)
	root := strings.Join(rootNames, ",")
	if len(rootNames) != 1 {
		return nil, errors.New("source archive must have one top-level directory")
	}
	manifest, err := GenerateOCBManifest(execution)
	if err != nil {
		return nil, err
	}
	manifestSum := sha256.Sum256([]byte(manifest))
	manifestSHA := hex.EncodeToString(manifestSum[:])
	canonical, _ := json.Marshal(struct {
		AcquisitionID, ExecutionID, PlanHash, ArchiveSHA, Root, ManifestSHA string
		Entries, Files, Directories, Bytes                                  int64
	}{acquisition.ID, execution.ID, execution.BuildPlanHash, acquisition.ArchiveSHA256, root, manifestSHA, entries, files, dirs, bytes})
	evidence := sha256.Sum256(canonical)
	evidenceHash := hex.EncodeToString(evidence[:])
	return &BuildSourceInspection{ID: "build-inspection-" + evidenceHash[:16], AcquisitionID: acquisition.ID, ExecutionID: execution.ID, BuildPlanHash: execution.BuildPlanHash, ArchiveSHA256: acquisition.ArchiveSHA256, EntryCount: entries, FileCount: files, DirectoryCount: dirs, UncompressedBytes: bytes, RootPrefix: root, ManifestYAML: manifest, ManifestSHA256: manifestSHA, Status: BuildSourceInspected, EvidenceHash: evidenceHash, InspectedBy: actor, InspectedAt: now.UTC()}, nil
}

var ocbModules = map[string]map[string]string{
	"receivers":  {"otlp": "go.opentelemetry.io/collector/receiver/otlpreceiver", "prometheus": "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"},
	"processors": {"batch": "go.opentelemetry.io/collector/processor/batchprocessor", "memory_limiter": "go.opentelemetry.io/collector/processor/memorylimiterprocessor"},
	"exporters":  {"otlp": "go.opentelemetry.io/collector/exporter/otlpexporter", "otlphttp": "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otlphttpexporter", "debug": "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/debugexporter"},
	"extensions": {"health_check": "github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension", "pprof": "github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension", "zpages": "go.opentelemetry.io/collector/extension/zpagesextension"},
	"connectors": {"forward": "go.opentelemetry.io/collector/connector/forwardconnector", "spanmetrics": "github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"},
}

// ReviewedOCBModules returns the de-duplicated, sorted Go module allow-list
// used by both manifest generation and the offline builder-image pipeline.
func ReviewedOCBModules() []string {
	seen := map[string]bool{}
	for _, kind := range ocbModules {
		for _, module := range kind {
			seen[module] = true
		}
	}
	modules := make([]string, 0, len(seen))
	for module := range seen {
		modules = append(modules, module)
	}
	sort.Strings(modules)
	return modules
}

func GenerateOCBManifest(execution *BuildExecution) (string, error) {
	if execution == nil {
		return "", errors.New("build execution is required")
	}
	var m BuildManifest
	if err := json.Unmarshal([]byte(execution.ManifestJSON), &m); err != nil {
		return "", errors.New("build execution manifest is invalid")
	}
	version := strings.TrimPrefix(strings.TrimSpace(m.CollectorVersion), "v")
	if version == "" {
		return "", errors.New("Collector version is required")
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "dist:\n  module: fleetamp.local/collector/%s\n  name: fleetamp-collector\n  description: FleetAMP governed Collector build %s\n  output_path: ./_build\n", execution.BuildPlanHash[:16], execution.BuildPlanHash[:16])
	types := []struct {
		name   string
		values []string
	}{{"receivers", m.Components.Receivers}, {"processors", m.Components.Processors}, {"exporters", m.Components.Exporters}, {"extensions", m.Components.Extensions}, {"connectors", m.Components.Connectors}}
	for _, kind := range types {
		if len(kind.values) == 0 {
			continue
		}
		fmt.Fprintf(b, "%s:\n", kind.name)
		values := append([]string(nil), kind.values...)
		sort.Strings(values)
		for _, name := range values {
			module, ok := ocbModules[kind.name][name]
			if !ok {
				return "", fmt.Errorf("unsupported %s component %q; add it to the reviewed component catalog", strings.TrimSuffix(kind.name, "s"), name)
			}
			fmt.Fprintf(b, "  - gomod: %s v%s\n", module, version)
		}
	}
	return b.String(), nil
}
