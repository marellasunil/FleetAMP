package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseFlexibleGroupSelectorForm(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(
		"name=Payments+EU&selector_key=team&selector_value=payments&selector_key=region&selector_value=eu-west-1",
	))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	selector, err := parseGroupSelectorForm(request)
	if err != nil {
		t.Fatal(err)
	}
	if selector["team"] != "payments" || selector["region"] != "eu-west-1" || len(selector) != 2 {
		t.Fatalf("unexpected selector: %#v", selector)
	}
}

func TestParseFlexibleGroupSelectorRejectsDuplicateKey(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(
		"selector_key=team&selector_value=payments&selector_key=team&selector_value=platform",
	))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := parseGroupSelectorForm(request); err == nil {
		t.Fatal("duplicate selector key was accepted")
	}
}

func TestSelectorsOverlap(t *testing.T) {
	tests := []struct {
		name        string
		left, right map[string]string
		wantOverlap bool
	}{
		{"same membership", map[string]string{"team": "payments"}, map[string]string{"team": "payments", "environment": "prod"}, true},
		{"conflicting shared key", map[string]string{"team": "payments"}, map[string]string{"team": "platform"}, false},
		{"no shared key", map[string]string{"team": "payments"}, map[string]string{"region": "eu"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectorsOverlap(test.left, test.right); got != test.wantOverlap {
				t.Fatalf("selectorsOverlap() = %v, want %v", got, test.wantOverlap)
			}
		})
	}
}

func TestNewValidatedGroupKeepsExplicitNameAndFlexibleKeys(t *testing.T) {
	group, err := newValidatedGroup(groupRequest{
		Name:     "Payments EU production",
		Selector: map[string]string{"team": "payments", "cloud.region": "eu-west-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if group.Name != "Payments EU production" || group.Selector["cloud.region"] != "eu-west-1" {
		t.Fatalf("unexpected group: %#v", group)
	}
}
