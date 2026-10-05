package main

import (
	"net/url"
	"testing"
)

func TestFilterFleetAdoptionRows(t *testing.T) {
	rows := []fleetAdoptionRow{
		{AgentName: "payments-prod", GroupID: "payments", GroupName: "Payments API", PatternName: "Linux host", State: "aligned"},
		{AgentName: "orders-prod", GroupID: "orders", GroupName: "Orders API", State: "custom", Attention: true},
	}
	if got := filterFleetAdoptionRows(rows, "payments", "aligned", "payments"); len(got) != 1 || got[0].AgentName != "payments-prod" {
		t.Fatalf("filtered rows=%#v", got)
	}
	if got := filterFleetAdoptionRows(rows, "", "attention", ""); len(got) != 1 || got[0].AgentName != "orders-prod" {
		t.Fatalf("attention rows=%#v", got)
	}
}

func TestFleetAdoptionPaginationURLsPreserveFilters(t *testing.T) {
	values := url.Values{"q": {"payment api"}, "status": {"candidate"}, "page": {"2"}}
	previous, next := fleetAdoptionPaginationURLs(values, 2, 4)
	for _, link := range []string{previous, next} {
		parsed, err := url.Parse(link)
		if err != nil {
			t.Fatalf("parse link: %v", err)
		}
		if parsed.Query().Get("q") != "payment api" || parsed.Query().Get("status") != "candidate" {
			t.Fatalf("filters lost in %q", link)
		}
	}
	if previous == next {
		t.Fatalf("pagination links must differ: %q", previous)
	}
}

func TestPositiveInt(t *testing.T) {
	if positiveInt("7", 1) != 7 || positiveInt("0", 1) != 1 || positiveInt("bad", 2) != 2 {
		t.Fatal("positiveInt fallback failed")
	}
}
