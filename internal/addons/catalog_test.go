package addons

import "testing"

func TestDefaultCatalogIncludesVendorDestinationsAndExternalCollector(t *testing.T) {
	catalog := NewDefaultCatalog()
	entries := catalog.List()
	if len(entries) != 3 {
		t.Fatalf("List() returned %d entries, want 3", len(entries))
	}
	alloy, ok := catalog.Get("alloy")
	if !ok || alloy.Bundled || alloy.Status != Planned {
		t.Fatalf("unexpected Alloy entry: %#v", alloy)
	}
	for _, id := range []string{"splunk", "grafana"} {
		entry, found := catalog.Get(id)
		if !found || entry.Status != Available || len(entry.Capabilities) == 0 || entry.Notice == "" {
			t.Fatalf("vendor destination %q is incomplete: %#v", id, entry)
		}
	}
}

func TestCatalogRejectsDuplicateIDs(t *testing.T) {
	_, err := NewCatalog(Entry{ID: "example", Name: "One"}, Entry{ID: "example", Name: "Two"})
	if err == nil {
		t.Fatal("NewCatalog() accepted duplicate IDs")
	}
}

func TestCatalogReturnsDefensiveCopies(t *testing.T) {
	catalog := NewDefaultCatalog()
	entry, _ := catalog.Get("alloy")
	entry.Capabilities[0] = WorkloadDeployment
	again, _ := catalog.Get("alloy")
	if again.Capabilities[0] == WorkloadDeployment {
		t.Fatal("Get() exposed mutable catalog state")
	}
}
