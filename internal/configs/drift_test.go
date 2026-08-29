// Tests semantic desired/effective configuration drift detection and diagnostics.
package configs

import "testing"

func TestCompareDesiredEffectiveInSyncWithExtraEffectiveConfig(t *testing.T) {
	desired := "service:\n  telemetry:\n    logs:\n      level: info\n"
	effective := "receivers:\n  otlp: {}\nservice:\n  telemetry:\n    logs:\n      level: info\n"
	result := CompareDesiredEffective(desired, effective)
	if result.Status != DriftInSync || !result.InSync || len(result.Differences) != 0 {
		t.Fatalf("expected in_sync, got %#v", result)
	}
}

func TestCompareDesiredEffectiveReportsValuePath(t *testing.T) {
	desired := "service:\n  telemetry:\n    logs:\n      level: info\n"
	effective := "service:\n  telemetry:\n    logs:\n      level: debug\n"
	result := CompareDesiredEffective(desired, effective)
	if result.Status != DriftDetected || len(result.Differences) != 1 {
		t.Fatalf("expected one drift difference, got %#v", result)
	}
	diff := result.Differences[0]
	if diff.Path != "service.telemetry.logs.level" || diff.Kind != "value_mismatch" || diff.Desired != "info" || diff.Effective != "debug" {
		t.Fatalf("unexpected difference: %#v", diff)
	}
}

func TestCompareDesiredEffectiveReportsMissingPath(t *testing.T) {
	result := CompareDesiredEffective("processors:\n  batch:\n    timeout: 5s\n", "processors:\n  batch: {}\n")
	if result.Status != DriftDetected || len(result.Differences) != 1 {
		t.Fatalf("expected missing setting, got %#v", result)
	}
	if result.Differences[0].Path != "processors.batch.timeout" || result.Differences[0].Kind != "missing" {
		t.Fatalf("unexpected difference: %#v", result.Differences[0])
	}
}

func TestCompareDesiredEffectiveUnknownWithoutEffective(t *testing.T) {
	result := CompareDesiredEffective("service: {}\n", "")
	if result.Status != DriftUnknown || result.InSync {
		t.Fatalf("expected unknown, got %#v", result)
	}
}
