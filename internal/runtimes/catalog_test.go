package runtimes

import (
	"errors"
	"testing"
)

func TestDefaultRegistryDescribesSupportedRuntimes(t *testing.T) {
	r := NewDefaultRegistry()
	listed := r.List()
	if len(listed) != 3 {
		t.Fatalf("List() returned %d providers, want 3", len(listed))
	}
	for _, runtimeType := range []Type{OTelCollector, OTelCollectorKubernetes, GrafanaAlloy} {
		descriptor, ok := r.Get(runtimeType)
		if !ok {
			t.Fatalf("Get(%q) did not find the provider", runtimeType)
		}
		if descriptor.ConfigFormat == "" || len(descriptor.ManagementModes) == 0 || len(descriptor.DeploymentMethods) == 0 || len(descriptor.Capabilities) == 0 {
			t.Fatalf("Get(%q) returned an incomplete descriptor: %#v", runtimeType, descriptor)
		}
	}
}

func TestRegistryRejectsDuplicateProviders(t *testing.T) {
	descriptor := Descriptor{Type: OTelCollector, Name: "Collector"}
	_, err := NewRegistry(staticProvider{descriptor}, staticProvider{descriptor})
	if !errors.Is(err, ErrDuplicateProvider) {
		t.Fatalf("NewRegistry() error = %v, want %v", err, ErrDuplicateProvider)
	}
}

func TestRegistryReturnsDefensiveCopies(t *testing.T) {
	r := NewDefaultRegistry()
	descriptor, _ := r.Get(GrafanaAlloy)
	descriptor.ManagementModes[0] = "changed"
	descriptor.Capabilities[0].Name = "changed"

	again, _ := r.Get(GrafanaAlloy)
	if again.ManagementModes[0] == "changed" || again.Capabilities[0].Name == "changed" {
		t.Fatal("Get() exposed mutable catalog state")
	}
}

func TestRegistryUnknownRuntime(t *testing.T) {
	if _, ok := NewDefaultRegistry().Get(Type("missing")); ok {
		t.Fatal("Get() found an unknown runtime")
	}
}
