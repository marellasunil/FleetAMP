package integrations

import "testing"

func TestConnectionRejectsRawSecretsAndPathEscape(t *testing.T) {
	base := Connection{Name: "prod", Provider: GitHub, Organization: "acme", Repository: "telemetry", Branch: "main", AllowedRoot: "fleetamp/groups", Mode: ModePullRequest, CredentialRef: "secret://github/prod", GroupIDs: []string{"payments"}, Enabled: true}
	c, e := NewConnection(base, "admin")
	if e != nil {
		t.Fatal(e)
	}
	if got, e := c.ResolvePath("payments/change.json"); e != nil || got != "fleetamp/groups/payments/change.json" {
		t.Fatalf("path=%q err=%v", got, e)
	}
	base.CredentialRef = "raw-token"
	if _, e := NewConnection(base, "admin"); e == nil {
		t.Fatal("raw credential accepted")
	}
	if _, e := c.ResolvePath("../escape"); e == nil {
		t.Fatal("path escape accepted")
	}
}

func TestConnectionRejectsUnsafeProviderBaseURL(t *testing.T) {
	base := Connection{Name: "prod", Provider: GitHub, Organization: "acme", Repository: "telemetry", Branch: "main", AllowedRoot: "fleetamp/groups", Mode: ModePullRequest, CredentialRef: "secret://github/prod", GroupIDs: []string{"payments"}, Enabled: true}
	for _, unsafe := range []string{"http://git.example.com", "https://user:password@git.example.com", "https://git.example.com?token=secret", "https://git.example.com#fragment"} {
		base.BaseURL = unsafe
		if _, err := NewConnection(base, "admin"); err == nil {
			t.Errorf("unsafe base URL accepted: %s", unsafe)
		}
	}
}
