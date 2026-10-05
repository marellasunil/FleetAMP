package addons

import "testing"

func TestDefaultCatalogIsExternalAndReadOnly(t *testing.T) {
	catalog := NewDefaultCatalog()
	entries := catalog.List()
	if len(entries) != 1 {
		t.Fatalf("List() returned %d entries, want 1", len(entries))
	}
	entry := entries[0]
	if entry.ID != "alloy" || entry.Bundled || entry.Status != Planned {
		t.Fatalf("unexpected default entry: %#v", entry)
	}
	if entry.RepositoryURL == "" || len(entry.Capabilities) == 0 || entry.Notice == "" {
		t.Fatalf("default entry is incomplete: %#v", entry)
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
