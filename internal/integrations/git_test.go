package integrations

import "testing"

func TestDefaultGitCatalogIncludesSupportedProviders(t *testing.T) {
	catalog := NewDefaultGitCatalog()
	if got := len(catalog.List()); got != 3 {
		t.Fatalf("List() returned %d providers, want 3", got)
	}
	for _, id := range []ProviderID{GitHub, GitLab, AzureDevOps} {
		provider, ok := catalog.Get(id)
		if !ok {
			t.Fatalf("Get(%q) did not find the provider", id)
		}
		if len(provider.Capabilities) == 0 || len(provider.SupportedModes) == 0 || len(provider.AuthOptions) == 0 {
			t.Fatalf("provider %q is incomplete: %#v", id, provider)
		}
	}
}

func TestGitCatalogRejectsDuplicateProviders(t *testing.T) {
	_, err := NewCatalog(Provider{ID: GitHub, Name: "One"}, Provider{ID: GitHub, Name: "Two"})
	if err == nil {
		t.Fatal("NewCatalog() accepted duplicate providers")
	}
}

func TestGitCatalogReturnsDefensiveCopies(t *testing.T) {
	catalog := NewDefaultGitCatalog()
	provider, _ := catalog.Get(GitHub)
	provider.AuthOptions[0] = "changed"
	provider.Capabilities[0] = ReportStatus
	again, _ := catalog.Get(GitHub)
	if again.AuthOptions[0] == "changed" || again.Capabilities[0] == ReportStatus {
		t.Fatal("Get() exposed mutable catalog state")
	}
}
