package addons

import "testing"

func TestDefaultCatalogUsesGenericIntegrationContracts(t *testing.T) {
	catalog := NewDefaultCatalog()
	entries := catalog.List()
	if len(entries) != 3 {
		t.Fatalf("List() returned %d entries, want 3", len(entries))
	}
	for _, id := range []string{"otlp-destination", "prometheus-remote-write", "external-collector-adapter"} {
		entry, found := catalog.Get(id)
		if !found || entry.Kind == "" || len(entry.Protocols) == 0 || len(entry.Signals) == 0 || len(entry.Capabilities) == 0 || entry.Notice == "" {
			t.Fatalf("integration contract %q is incomplete: %#v", id, entry)
		}
	}
	if adapter, _ := catalog.Get("external-collector-adapter"); adapter.Status != Planned {
		t.Fatalf("external Collector adapter status = %q, want %q", adapter.Status, Planned)
	}
}

func TestCatalogRequiresGenericIdentity(t *testing.T) {
	for name, entries := range map[string][]Entry{
		"missing id":   {{Name: "One", Kind: "Destination"}},
		"missing name": {{ID: "one", Kind: "Destination"}},
		"missing kind": {{ID: "one", Name: "One"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewCatalog(entries...); err == nil {
				t.Fatal("NewCatalog() accepted an incomplete entry")
			}
		})
	}
}

func TestCatalogRejectsDuplicateIDs(t *testing.T) {
	_, err := NewCatalog(
		Entry{ID: "example", Name: "One", Kind: "Destination"},
		Entry{ID: "example", Name: "Two", Kind: "Destination"},
	)
	if err == nil {
		t.Fatal("NewCatalog() accepted duplicate IDs")
	}
}

func TestCatalogReturnsDefensiveCopies(t *testing.T) {
	catalog := NewDefaultCatalog()
	entry, _ := catalog.Get("otlp-destination")
	entry.Protocols[0] = "changed"
	entry.Signals[0] = "changed"
	entry.Capabilities[0] = WorkloadDeployment

	again, _ := catalog.Get("otlp-destination")
	if again.Protocols[0] == "changed" || again.Signals[0] == "changed" || again.Capabilities[0] == WorkloadDeployment {
		t.Fatal("Get() exposed mutable catalog state")
	}
}
