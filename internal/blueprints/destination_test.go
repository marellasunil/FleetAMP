package blueprints

import "testing"

func TestDestinationProfileAllowsGroup(t *testing.T) {
	profile := NewDestinationProfile("OTLP", "Production", "otlphttp/prod", "endpoint: https://example.invalid")
	if !profile.AllowsGroup("any-group") {
		t.Fatal("organization destination should allow every group")
	}
	profile.Visibility = "restricted"
	profile.GroupIDs = []string{"payments-prod"}
	if !profile.AllowsGroup("payments-prod") {
		t.Fatal("restricted destination should allow an explicitly approved group")
	}
	if profile.AllowsGroup("security-prod") {
		t.Fatal("restricted destination must reject an unapproved group")
	}
	profile.Enabled = false
	if profile.AllowsGroup("payments-prod") {
		t.Fatal("disabled destination must reject every group")
	}
}
