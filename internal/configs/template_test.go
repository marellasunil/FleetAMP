package configs

import (
	"testing"
	"time"
)

func TestConfigurationTemplateIsImmutableAndGroupScoped(t *testing.T) {
	content := "receivers:\n  otlp: {}\nservice:\n  pipelines: {}\n"
	item, err := NewTemplate("Standard", "1.0.0", "baseline", content, TemplateSelected, []string{"payments", "payments", "orders"}, "config-1", "admin", time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !item.AllowsGroup("payments") || item.AllowsGroup("unknown") {
		t.Fatalf("unexpected template group scope: %#v", item.GroupIDs)
	}
	if len(item.GroupIDs) != 2 || item.ContentHash == "" || item.ID == "" {
		t.Fatalf("template was not normalized: %#v", item)
	}
	again, err := NewTemplate("Standard", "1.0.0", "different description", content, TemplateSelected, []string{"orders", "payments"}, "config-2", "other", time.Unix(2, 0))
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != item.ID {
		t.Fatal("template identity changed for equivalent immutable content and scope")
	}
}

func TestOrganizationTemplateAllowsEveryGroup(t *testing.T) {
	item, err := NewTemplate("Global", "1", "", "service: {}", TemplateOrganization, []string{"ignored"}, "", "admin", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(item.GroupIDs) != 0 || !item.AllowsGroup("any-group") {
		t.Fatalf("organization template scope is incorrect: %#v", item)
	}
}
