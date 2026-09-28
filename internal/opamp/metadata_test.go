package opamp

import "testing"

func TestSplitFleetAMPMetadataAcceptsFlexibleGroupKeys(t *testing.T) {
	groups, labels, unknown := splitFleetAMPMetadata(map[string]string{
		"fleetamp.group.team":         "payments",
		"fleetamp.group.cloud.region": "eu-west-1",
		"fleetamp.label.release-ring": "canary",
	})
	if groups["team"] != "payments" || groups["cloud.region"] != "eu-west-1" {
		t.Fatalf("unexpected group fields: %#v", groups)
	}
	if labels["release-ring"] != "canary" {
		t.Fatalf("unexpected labels: %#v", labels)
	}
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown fields: %#v", unknown)
	}
}
